// Package logging builds the zap logger every Poro Go service uses.
package logging

import (
	"fmt"

	"go.uber.org/zap"
)

// New returns a JSON production logger, or a console logger when dev is true.
// Stack traces are disabled: errors carry request_id and the wrapped cause.
// Every entry goes through Redact: secrets, tokens, emails and phone numbers
// never reach the logs.
func New(service, level string, dev bool) (*zap.Logger, error) {
	zc := zap.NewProductionConfig()
	if dev {
		zc = zap.NewDevelopmentConfig()
	}
	zc.DisableStacktrace = true
	if err := zc.Level.UnmarshalText([]byte(level)); err != nil {
		return nil, fmt.Errorf("parse log level %q: %w", level, err)
	}
	log, err := zc.Build(zap.WrapCore(Redact))
	if err != nil {
		return nil, fmt.Errorf("build logger: %w", err)
	}
	return log.With(zap.String("service", service)), nil
}
