package logging

import (
	"regexp"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Redacted replaces a value that must never reach the logs.
const Redacted = "[REDACTED]"

// sensitiveKeys are field names whose value is always replaced, whatever it
// contains. Matching is case-insensitive on the whole key or its last segment
// after a dot or an underscore.
var sensitiveKeys = map[string]bool{
	"password": true, "new_password": true, "secret": true, "token": true, "access_token": true,
	"refresh_token": true, "id_token": true, "fcm_token": true, "authorization": true, "cookie": true,
	"api_key": true, "apikey": true, "otp": true, "otp_code": true, "private_key": true, "secret_access_key": true,
}

var (
	jwtPattern    = regexp.MustCompile(`eyJ[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]{5,}\.[A-Za-z0-9_-]{5,}`)
	bearerPattern = regexp.MustCompile(`(?i)\b(bearer|basic)\s+[A-Za-z0-9._~+/=-]{8,}`)
	emailPattern  = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)
	// International numbers (E.164) and long digit runs that look like phones.
	phonePattern = regexp.MustCompile(`\+\d{8,15}\b`)
)

// Scrub removes tokens, emails and phone numbers from free text such as
// error messages, which may quote a request or a provider response.
func Scrub(s string) string {
	if s == "" {
		return s
	}
	s = jwtPattern.ReplaceAllString(s, Redacted)
	s = bearerPattern.ReplaceAllString(s, "$1 "+Redacted)
	s = emailPattern.ReplaceAllString(s, Redacted)
	return phonePattern.ReplaceAllString(s, Redacted)
}

func sensitive(key string) bool {
	k := strings.ToLower(key)
	if sensitiveKeys[k] {
		return true
	}
	if i := strings.LastIndexAny(k, "._"); i >= 0 && sensitiveKeys[k[i+1:]] {
		return true
	}
	return false
}

// redactCore scrubs every entry before the wrapped core encodes it.
type redactCore struct{ zapcore.Core }

// Redact wraps a core so that no field or message carries a secret or a
// contact detail. Services keep masking on purpose (for instance the last
// digits of a phone); this is the safety net for what they forget.
func Redact(core zapcore.Core) zapcore.Core { return redactCore{core} }

func (c redactCore) With(fields []zapcore.Field) zapcore.Core {
	return redactCore{c.Core.With(scrubFields(fields))}
}

func (c redactCore) Check(entry zapcore.Entry, checked *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Enabled(entry.Level) {
		return checked.AddCore(entry, c)
	}
	return checked
}

func (c redactCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	entry.Message = Scrub(entry.Message)
	return c.Core.Write(entry, scrubFields(fields))
}

func scrubFields(fields []zapcore.Field) []zapcore.Field {
	out := make([]zapcore.Field, len(fields))
	for i, f := range fields {
		out[i] = scrubField(f)
	}
	return out
}

func scrubField(f zapcore.Field) zapcore.Field {
	if sensitive(f.Key) {
		return zap.String(f.Key, Redacted)
	}
	switch f.Type {
	case zapcore.StringType:
		f.String = Scrub(f.String)
	case zapcore.ErrorType:
		if err, ok := f.Interface.(error); ok && err != nil {
			return zap.String(f.Key, Scrub(err.Error()))
		}
	case zapcore.StringerType:
		if s, ok := f.Interface.(interface{ String() string }); ok && s != nil {
			return zap.String(f.Key, Scrub(s.String()))
		}
	}
	return f
}
