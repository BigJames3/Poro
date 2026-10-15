package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"

	"github.com/poro/auth/internal/config"
)

// ErrSMSBudgetExhausted is returned once the daily SMS budget of the whole
// platform is spent: no code is sent until the next UTC day (ADR-0017). It
// answers like a provider outage.
var ErrSMSBudgetExhausted = fmt.Errorf("%w: daily sms budget exhausted", ErrSMSUnavailable)

// SMSBudget bounds what SMS pumping can cost (ADR-0017). Spend reserves one SMS
// against every cap or refuses it; Refund gives the reservation back when the
// provider did not send it.
type SMSBudget interface {
	Spend(ctx context.Context, phone, country string) error
	Refund(ctx context.Context, phone, country string) error
	Delivered(country string)
}

const smsBudgetKeyPrefix = "auth:sms:"

// The caps, checked in this order. A phone and a number range answer 429 with
// the end of their window; a country too; the global cap stops every SMS.
const (
	capPhoneDaily = iota
	capPrefixHourly
	capCountryHourly
	capGlobalDaily
)

var capNames = [...]string{"phone_daily", "prefix_hourly", "country_hourly", "global_daily"}

// spendScript checks every cap, then counts the SMS against all of them, in
// one atomic step: concurrent requests cannot overshoot a cap together.
// KEYS are the counters; ARGV holds the limits, then the TTLs in milliseconds.
// It answers 0 when the SMS is allowed, or the 1-based index of the full cap.
var spendScript = redis.NewScript(`
local n = #KEYS
for i = 1, n do
  if tonumber(redis.call('GET', KEYS[i]) or '0') >= tonumber(ARGV[i]) then
    return i
  end
end
for i = 1, n do
  if redis.call('INCR', KEYS[i]) == 1 then
    redis.call('PEXPIRE', KEYS[i], ARGV[n + i])
  end
end
return 0
`)

// refundScript gives one SMS back to each counter that still exists.
var refundScript = redis.NewScript(`
for i = 1, #KEYS do
  if tonumber(redis.call('GET', KEYS[i]) or '0') > 0 then
    redis.call('DECR', KEYS[i])
  end
end
return 0
`)

type redisSMSBudget struct {
	rdb     *redis.Client
	limits  [4]int
	now     func() time.Time
	sent    *prometheus.CounterVec
	blocked *prometheus.CounterVec
}

// NewRedisSMSBudget builds the budget. With a registry it exports
// auth_sms_sent_total{country} and auth_sms_blocked_total{reason}.
func NewRedisSMSBudget(cfg *config.Config, rdb *redis.Client, registry prometheus.Registerer) (SMSBudget, error) {
	if rdb == nil {
		return nil, errors.New("sms budget: redis is required")
	}
	limits := [4]int{cfg.SMSDailyLimitPerPhone, cfg.SMSHourlyLimitPerPrefix, cfg.SMSHourlyLimitPerCountry, cfg.SMSDailyLimit}
	for i, limit := range limits {
		if limit <= 0 {
			return nil, fmt.Errorf("sms budget: the %s cap must be positive, got %d", capNames[i], limit)
		}
	}
	b := &redisSMSBudget{
		rdb:    rdb,
		limits: limits,
		now:    func() time.Time { return time.Now().UTC() },
		sent: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "auth_sms_sent_total",
			Help: "OTP SMS handed to the provider, by calling code.",
		}, []string{"country"}),
		blocked: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "auth_sms_blocked_total",
			Help: "OTP SMS refused by the SMS budget, by cap.",
		}, []string{"reason"}),
	}
	if registry != nil {
		for _, c := range []prometheus.Collector{b.sent, b.blocked} {
			if err := registry.Register(c); err != nil {
				return nil, fmt.Errorf("sms budget: register metrics: %w", err)
			}
		}
	}
	return b, nil
}

// window is one cap's counter: its key, the end of its fixed UTC window and
// how long the key must live.
type window struct {
	key string
	end time.Time
}

func (b *redisSMSBudget) windows(phone, country string) [4]window {
	now := b.now()
	day := now.Truncate(24 * time.Hour)
	hour := now.Truncate(time.Hour)
	dayEnd, hourEnd := day.Add(24*time.Hour), hour.Add(time.Hour)
	d, h := day.Format("20060102"), hour.Format("2006010215")
	return [4]window{
		{smsBudgetKeyPrefix + "phone:" + phone + ":" + d, dayEnd},
		{smsBudgetKeyPrefix + "prefix:" + numberRange(phone) + ":" + h, hourEnd},
		{smsBudgetKeyPrefix + "country:" + country + ":" + h, hourEnd},
		{smsBudgetKeyPrefix + "global:" + d, dayEnd},
	}
}

func (b *redisSMSBudget) Spend(ctx context.Context, phone, country string) error {
	windows := b.windows(phone, country)
	keys := make([]string, len(windows))
	args := make([]any, 0, 2*len(windows))
	for i, w := range windows {
		keys[i] = w.key
		args = append(args, b.limits[i])
	}
	for _, w := range windows {
		// A minute past the window end, so a key never outlives its window
		// by much and never expires inside it.
		args = append(args, (w.end.Sub(b.now()) + time.Minute).Milliseconds())
	}
	full, err := spendScript.Run(ctx, b.rdb, keys, args...).Int()
	if err != nil {
		return fmt.Errorf("sms budget: %w", err)
	}
	if full == 0 {
		return nil
	}
	reached := full - 1
	b.blocked.WithLabelValues(capNames[reached]).Inc()
	if reached == capGlobalDaily {
		return ErrSMSBudgetExhausted
	}
	return &ThrottledError{RetryAfter: windows[reached].end.Sub(b.now())}
}

func (b *redisSMSBudget) Refund(ctx context.Context, phone, country string) error {
	windows := b.windows(phone, country)
	keys := make([]string, len(windows))
	for i, w := range windows {
		keys[i] = w.key
	}
	if err := refundScript.Run(ctx, b.rdb, keys).Err(); err != nil {
		return fmt.Errorf("sms budget refund: %w", err)
	}
	return nil
}

func (b *redisSMSBudget) Delivered(country string) {
	b.sent.WithLabelValues(country).Inc()
}

// numberRange is the phone number without its last three digits: SMS pumping
// rents ranges of consecutive premium numbers.
func numberRange(phone string) string {
	if len(phone) <= 6 {
		return phone
	}
	return phone[:len(phone)-3]
}
