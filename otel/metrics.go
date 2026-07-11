package otelresilium

import (
	"context"

	"github.com/sinashahoveisi/resilium"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Metrics returns resilium.Hooks that record retry attempts and circuit
// breaker state transitions on the given OpenTelemetry meter.
//
// Operation duration histograms are not included because the core Hooks
// type has no duration callback; that can be added in a follow-up.
func Metrics(meter metric.Meter) resilium.Hooks {
	retryCounter, _ := meter.Int64Counter(
		"resilium.retry.attempts",
		metric.WithDescription("Number of failed attempts that triggered a retry hook"),
	)
	openCounter, _ := meter.Int64Counter(
		"resilium.circuit_breaker.open",
		metric.WithDescription("Number of times a circuit breaker opened"),
	)
	closeCounter, _ := meter.Int64Counter(
		"resilium.circuit_breaker.close",
		metric.WithDescription("Number of times a circuit breaker closed"),
	)
	rateLimitedCounter, _ := meter.Int64Counter(
		"resilium.rate_limited",
		metric.WithDescription("Number of calls rejected by rate limiting"),
	)

	return resilium.Hooks{
		OnRetry: func(attempt int, err error) {
			retryCounter.Add(context.Background(), 1,
				metric.WithAttributes(attribute.Int("attempt", attempt)))
		},
		OnCircuitOpen: func(name string) {
			attrs := []attribute.KeyValue{}
			if name != "" {
				attrs = append(attrs, attribute.String("name", name))
			}
			openCounter.Add(context.Background(), 1, metric.WithAttributes(attrs...))
		},
		OnCircuitClose: func(name string) {
			attrs := []attribute.KeyValue{}
			if name != "" {
				attrs = append(attrs, attribute.String("name", name))
			}
			closeCounter.Add(context.Background(), 1, metric.WithAttributes(attrs...))
		},
		OnRateLimited: func() {
			rateLimitedCounter.Add(context.Background(), 1)
		},
	}
}
