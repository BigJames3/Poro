// Package metrics exposes Prometheus metrics for HTTP traffic and lets services
// register their own collectors on the same registry.
package metrics

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics owns a registry with runtime collectors and HTTP RED metrics.
type Metrics struct {
	registry *prometheus.Registry
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
}

// New builds the registry. Labels use the route template, never the raw path,
// to keep cardinality bounded.
func New(service string) (*Metrics, error) {
	reg := prometheus.NewRegistry()
	constLabels := prometheus.Labels{"service": service}
	m := &Metrics{
		registry: reg,
		requests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name:        "http_requests_total",
			Help:        "HTTP requests by route and status.",
			ConstLabels: constLabels,
		}, []string{"method", "route", "status"}),
		duration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:        "http_request_duration_seconds",
			Help:        "HTTP request latency by route.",
			ConstLabels: constLabels,
			Buckets:     []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
		}, []string{"method", "route"}),
	}
	for _, c := range []prometheus.Collector{
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		m.requests,
		m.duration,
	} {
		if err := reg.Register(c); err != nil {
			return nil, fmt.Errorf("register collector: %w", err)
		}
	}
	return m, nil
}

// Registry lets a service register business metrics.
func (m *Metrics) Registry() *prometheus.Registry { return m.registry }

// Middleware records every request. It must run outside AccessLog, which
// resolves chain errors, so the recorded status is the one sent to the client.
func (m *Metrics) Middleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		route := c.Route().Path
		m.requests.WithLabelValues(c.Method(), route, strconv.Itoa(c.Response().StatusCode())).Inc()
		m.duration.WithLabelValues(c.Method(), route).Observe(time.Since(start).Seconds())
		return err
	}
}

// Handler serves the registry in the Prometheus text format.
func (m *Metrics) Handler() fiber.Handler {
	return adaptor.HTTPHandler(promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{}))
}
