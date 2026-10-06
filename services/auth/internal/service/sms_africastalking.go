package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/poro/auth/internal/config"
)

const (
	africasTalkingLiveURL    = "https://api.africastalking.com/version1/messaging"
	africasTalkingSandboxURL = "https://api.sandbox.africastalking.com/version1/messaging"
	africasTalkingTimeout    = 10 * time.Second
	maxProviderResponseBytes = 64 << 10
	maxProviderDetailLen     = 120
)

// Recipient status codes documented by Africa's Talking.
const (
	atStatusProcessed          = 100
	atStatusSent               = 101
	atStatusQueued             = 102
	atStatusInvalidPhoneNumber = 403
	atStatusUnsupportedNumber  = 404
	atStatusUserInBlacklist    = 406
)

type africasTalkingSender struct {
	client   *http.Client
	endpoint string
	username string
	apiKey   string
	senderID string
	log      *zap.Logger
}

func newAfricasTalkingSender(cfg *config.Config, client *http.Client, log *zap.Logger) *africasTalkingSender {
	endpoint := africasTalkingLiveURL
	if cfg.AfricasTalkingUsername == config.AfricasTalkingSandboxUser {
		endpoint = africasTalkingSandboxURL
	}
	return &africasTalkingSender{
		client:   client,
		endpoint: endpoint,
		username: cfg.AfricasTalkingUsername,
		apiKey:   cfg.AfricasTalkingAPIKey,
		senderID: cfg.AfricasTalkingSenderID,
		log:      log,
	}
}

type africasTalkingResponse struct {
	SMSMessageData struct {
		Message    string `json:"Message"`
		Recipients []struct {
			StatusCode int    `json:"statusCode"`
			Status     string `json:"status"`
			MessageID  string `json:"messageId"`
		} `json:"Recipients"`
	} `json:"SMSMessageData"`
}

// SendOTP sends one SMS. It is not retried: a timeout may still deliver the
// message, and a retry would send a second code to the user.
func (s *africasTalkingSender) SendOTP(ctx context.Context, phone, code string, ttl time.Duration) error {
	form := url.Values{}
	form.Set("username", s.username)
	form.Set("to", phone)
	form.Set("message", otpMessage(phone, code, ttl))
	form.Set("bulkSMSMode", "1")
	if s.senderID != "" {
		form.Set("from", s.senderID)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("%w: africastalking: build request: %v", ErrSMSUnavailable, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("apiKey", s.apiKey)

	res, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: africastalking: %v", ErrSMSUnavailable, err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, maxProviderResponseBytes))
	if err != nil {
		return fmt.Errorf("%w: africastalking: read response: %v", ErrSMSUnavailable, err)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("%w: africastalking: http %d", ErrSMSUnavailable, res.StatusCode)
	}

	var parsed africasTalkingResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return fmt.Errorf("%w: africastalking: decode response: %v", ErrSMSUnavailable, err)
	}
	recipients := parsed.SMSMessageData.Recipients
	if len(recipients) != 1 {
		return fmt.Errorf("%w: africastalking: %d recipients: %s", ErrSMSUnavailable, len(recipients), truncate(parsed.SMSMessageData.Message))
	}

	r := recipients[0]
	switch r.StatusCode {
	case atStatusProcessed, atStatusSent, atStatusQueued:
		s.log.Info("sms otp sent",
			zap.String("phone", MaskPhone(phone)),
			zap.String("provider", config.SMSProviderAfricasTalking),
			zap.String("message_id", r.MessageID),
			zap.String("status", r.Status),
		)
		return nil
	case atStatusInvalidPhoneNumber, atStatusUnsupportedNumber, atStatusUserInBlacklist:
		return fmt.Errorf("%w: africastalking: %d %s", ErrPhoneUnreachable, r.StatusCode, truncate(r.Status))
	default:
		return fmt.Errorf("%w: africastalking: %d %s", ErrSMSUnavailable, r.StatusCode, truncate(r.Status))
	}
}

// otpMessage writes the SMS in the main language of the destination country:
// English for Nigeria, French for the francophone launch markets.
func otpMessage(phone, code string, ttl time.Duration) string {
	minutes := int(math.Ceil(ttl.Minutes()))
	if strings.HasPrefix(phone, "+234") {
		return fmt.Sprintf("Your PORO code is %s. It expires in %d min. Never share it.", code, minutes)
	}
	return fmt.Sprintf("Votre code PORO : %s. Valable %d min. Ne le partagez jamais.", code, minutes)
}

func truncate(s string) string {
	if len(s) > maxProviderDetailLen {
		return s[:maxProviderDetailLen]
	}
	return s
}
