// Package events defines the envelope of every Poro domain event.
//
// One topic per event type, named poro.{domain}.{entity}.{action}, or
// poro.{domain}.{action} when the entity is the domain (e.g. poro.video.ready).
// The message key is the subject (the aggregate ID), so events of one aggregate stay ordered.
package events

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
)

var typePattern = regexp.MustCompile(`^poro(\.[a-z][a-z0-9_]*){2,3}$`)

// ErrInvalidEnvelope is returned for a message that is not a valid envelope.
var ErrInvalidEnvelope = errors.New("invalid event envelope")

// Envelope wraps the event data with the metadata consumers rely on.
// Additive changes to Data keep the version; a breaking change increments it.
type Envelope struct {
	ID         uuid.UUID       `json:"id"`
	Type       string          `json:"type"`
	Version    int             `json:"version"`
	Source     string          `json:"source"`
	Subject    string          `json:"subject"`
	OccurredAt time.Time       `json:"occurred_at"`
	Data       json.RawMessage `json:"data"`
}

// New builds an envelope with a UUIDv7 ID.
func New(eventType string, version int, source, subject string, data any, occurredAt time.Time) (Envelope, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return Envelope{}, fmt.Errorf("marshal event data: %w", err)
	}
	id, err := uuid.NewV7()
	if err != nil {
		return Envelope{}, fmt.Errorf("event id: %w", err)
	}
	env := Envelope{
		ID:         id,
		Type:       eventType,
		Version:    version,
		Source:     source,
		Subject:    subject,
		OccurredAt: occurredAt.UTC(),
		Data:       raw,
	}
	if err := env.Validate(); err != nil {
		return Envelope{}, err
	}
	return env, nil
}

// Validate checks the required metadata.
func (e Envelope) Validate() error {
	switch {
	case e.ID == uuid.Nil:
		return fmt.Errorf("%w: missing id", ErrInvalidEnvelope)
	case !typePattern.MatchString(e.Type):
		return fmt.Errorf("%w: type %q must match poro.{domain}.{entity}.{action}", ErrInvalidEnvelope, e.Type)
	case e.Version < 1:
		return fmt.Errorf("%w: version must be at least 1", ErrInvalidEnvelope)
	case e.Source == "":
		return fmt.Errorf("%w: missing source", ErrInvalidEnvelope)
	case e.Subject == "":
		return fmt.Errorf("%w: missing subject", ErrInvalidEnvelope)
	case e.OccurredAt.IsZero():
		return fmt.Errorf("%w: missing occurred_at", ErrInvalidEnvelope)
	case len(e.Data) == 0 || e.Data[0] != '{':
		return fmt.Errorf("%w: data must be a JSON object", ErrInvalidEnvelope)
	}
	return nil
}

// Topic is the Kafka topic of the event.
func (e Envelope) Topic() string { return e.Type }

// Decode parses and validates a message value.
func Decode(value []byte) (Envelope, error) {
	var env Envelope
	if err := json.Unmarshal(value, &env); err != nil {
		return Envelope{}, fmt.Errorf("%w: %w", ErrInvalidEnvelope, err)
	}
	if err := env.Validate(); err != nil {
		return Envelope{}, err
	}
	return env, nil
}

// DecodeData unmarshals the event data into v.
func (e Envelope) DecodeData(v any) error {
	if err := json.Unmarshal(e.Data, v); err != nil {
		return fmt.Errorf("decode %s data: %w", e.Type, err)
	}
	return nil
}

// DLQTopic is where a consumer parks the messages of topic it cannot process.
func DLQTopic(topic string) string { return topic + ".dlq" }
