package otelresilium_test

import (
	"context"
	"errors"
	"testing"

	otelresilium "github.com/sinashahoveisi/resilium/otel"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestMetricsCounterValues(t *testing.T) {
	reader := metric.NewManualReader()
	provider := metric.NewMeterProvider(metric.WithReader(reader))
	meter := provider.Meter("test")

	hooks := otelresilium.Metrics(meter)

	hooks.OnRetry(1, errors.New("fail"))
	hooks.OnRetry(2, errors.New("fail"))
	hooks.OnCircuitOpen("")
	hooks.OnCircuitClose("")
	hooks.OnRateLimited()
	hooks.OnRateLimited()

	var rm metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("Collect() error = %v", err)
	}

	if got := sumCounter("resilium.retry.attempts", rm); got != 2 {
		t.Fatalf("resilium.retry.attempts = %d, want 2", got)
	}
	if got := sumCounter("resilium.circuit_breaker.open", rm); got != 1 {
		t.Fatalf("resilium.circuit_breaker.open = %d, want 1", got)
	}
	if got := sumCounter("resilium.circuit_breaker.close", rm); got != 1 {
		t.Fatalf("resilium.circuit_breaker.close = %d, want 1", got)
	}
	if got := sumCounter("resilium.rate_limited", rm); got != 2 {
		t.Fatalf("resilium.rate_limited = %d, want 2", got)
	}
}

func sumCounter(name string, rm metricdata.ResourceMetrics) int64 {
	var total int64
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			sum, ok := m.Data.(metricdata.Sum[int64])
			if !ok {
				continue
			}
			for _, dp := range sum.DataPoints {
				total += dp.Value
			}
		}
	}
	return total
}
