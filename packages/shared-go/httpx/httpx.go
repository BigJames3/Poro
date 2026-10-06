// Package httpx holds the HTTP conventions shared by every Poro Go service:
// the {data, error, meta} envelope, stable error codes, request IDs and access logs.
package httpx

import (
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

const (
	// HeaderRequestID carries the correlation ID in both directions.
	HeaderRequestID = "X-Request-ID"
	// LocalRequestID is the Fiber local key for the correlation ID.
	LocalRequestID = "requestID"

	maxRequestIDLen = 128
)

// APIError is an error with a stable machine-readable code for clients.
// Cause is logged for server errors and never sent to the client.
type APIError struct {
	Status  int
	Code    string
	Message string
	Cause   error
}

func (e *APIError) Error() string {
	if e.Cause != nil {
		return e.Code + ": " + e.Message + ": " + e.Cause.Error()
	}
	return e.Code + ": " + e.Message
}

// Unwrap exposes the cause to errors.Is and errors.As.
func (e *APIError) Unwrap() error { return e.Cause }

// NewAPIError builds an APIError.
func NewAPIError(status int, code, message string) *APIError {
	return &APIError{Status: status, Code: code, Message: message}
}

// WithCause returns a copy of the error that carries the internal cause.
func (e *APIError) WithCause(err error) *APIError {
	cp := *e
	cp.Cause = err
	return &cp
}

// WriteData renders a success envelope.
func WriteData(c *fiber.Ctx, status int, data any) error {
	return c.Status(status).JSON(fiber.Map{
		"data":  data,
		"error": nil,
		"meta":  fiber.Map{"request_id": RequestIDFrom(c)},
	})
}

// RequestID reuses a well-formed incoming X-Request-ID or generates a UUIDv7,
// and echoes it on the response.
func RequestID() fiber.Handler {
	return func(c *fiber.Ctx) error {
		id := c.Get(HeaderRequestID)
		if !validRequestID(id) {
			id = uuid.Must(uuid.NewV7()).String()
		}
		c.Locals(LocalRequestID, id)
		c.Set(HeaderRequestID, id)
		return c.Next()
	}
}

// RequestIDFrom returns the correlation ID stored by RequestID.
func RequestIDFrom(c *fiber.Ctx) string {
	id, _ := c.Locals(LocalRequestID).(string)
	return id
}

// AccessLog writes one structured line per request. It resolves chain errors
// through the app error handler first, so the logged status is the real one.
func AccessLog(log *zap.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		if chainErr := c.Next(); chainErr != nil {
			if err := c.App().ErrorHandler(c, chainErr); err != nil {
				_ = c.SendStatus(fiber.StatusInternalServerError)
			}
		}
		status := c.Response().StatusCode()
		fields := []zap.Field{
			zap.String("request_id", RequestIDFrom(c)),
			zap.String("method", c.Method()),
			zap.String("route", c.Route().Path),
			zap.Int("status", status),
			zap.Duration("latency", time.Since(start)),
			zap.String("ip", c.IP()),
		}
		if sc := trace.SpanContextFromContext(c.UserContext()); sc.IsValid() {
			fields = append(fields, zap.String("trace_id", sc.TraceID().String()))
		}
		switch {
		case status >= fiber.StatusInternalServerError:
			log.Error("http request", fields...)
		case isProbe(c.Path()):
			log.Debug("http request", fields...)
		default:
			log.Info("http request", fields...)
		}
		return nil
	}
}

// ErrorHandler renders every error in the {data, error, meta} envelope.
// Internal errors are logged with detail and answered with a generic message.
func ErrorHandler(log *zap.Logger) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		status, code, msg := fiber.StatusInternalServerError, "internal_error", "internal server error"
		var apiErr *APIError
		var fe *fiber.Error
		switch {
		case errors.As(err, &apiErr):
			status, code, msg = apiErr.Status, apiErr.Code, apiErr.Message
		case errors.As(err, &fe):
			status, code, msg = fe.Code, codeForStatus(fe.Code), fe.Message
		}
		if status >= fiber.StatusInternalServerError {
			log.Error("request failed",
				zap.String("request_id", RequestIDFrom(c)),
				zap.String("method", c.Method()),
				zap.String("path", c.Path()),
				zap.Error(err),
			)
		}
		return c.Status(status).JSON(fiber.Map{
			"data":  nil,
			"error": fiber.Map{"code": code, "message": msg},
			"meta":  fiber.Map{"request_id": RequestIDFrom(c)},
		})
	}
}

func codeForStatus(status int) string {
	switch status {
	case fiber.StatusBadRequest:
		return "invalid_request"
	case fiber.StatusUnauthorized:
		return "unauthorized"
	case fiber.StatusForbidden:
		return "forbidden"
	case fiber.StatusNotFound:
		return "not_found"
	case fiber.StatusMethodNotAllowed:
		return "method_not_allowed"
	case fiber.StatusConflict:
		return "conflict"
	case fiber.StatusRequestEntityTooLarge:
		return "payload_too_large"
	case fiber.StatusTooManyRequests:
		return "rate_limited"
	case fiber.StatusServiceUnavailable:
		return "unavailable"
	default:
		if status >= fiber.StatusInternalServerError {
			return "internal_error"
		}
		return "error"
	}
}

func isProbe(path string) bool {
	return strings.HasPrefix(path, "/health") || path == "/metrics"
}

func validRequestID(id string) bool {
	if id == "" || len(id) > maxRequestIDLen {
		return false
	}
	for _, r := range id {
		ok := r == '-' || r == '_' || r == '.' ||
			(r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
		if !ok {
			return false
		}
	}
	return true
}
