package integration_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sinashahoveisi/resilium"
	"github.com/sinashahoveisi/resilium/circuitbreaker"
	"github.com/sinashahoveisi/resilium/retry"
)

func TestIntegrationLoadConcurrent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping load test in -short mode")
	}

	const (
		workers       = 200
		loadDuration  = 2 * time.Second
		p99Multiplier = 10
	)

	var serverHits atomic.Int64
	fastServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverHits.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	}))
	defer fastServer.Close()

	policy := resilium.New(
		resilium.WithRateLimit(500, 50),
		resilium.WithTimeout(100*time.Millisecond),
		resilium.WithRetry(retry.Config{
			MaxAttempts: 3,
			Backoff:     retry.FixedBackoff(time.Millisecond),
		}),
		resilium.WithCircuitBreaker(circuitbreaker.Config{
			FailureThreshold: 0.5,
			MinRequests:      20,
			OpenDuration:     50 * time.Millisecond,
			WindowSize:       30,
		}),
	)

	runLoad := func(label string, fn func(context.Context) error) (int64, int64, int64, int64, []time.Duration) {
		ctx, cancel := context.WithTimeout(context.Background(), loadDuration)
		defer cancel()

		var (
			wg          sync.WaitGroup
			total       atomic.Int64
			successes   atomic.Int64
			failures    atomic.Int64
			rateLimited atomic.Int64
			latenciesMu sync.Mutex
			latencies   []time.Duration
			panicCh     = make(chan any, workers)
		)

		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() {
					if v := recover(); v != nil {
						panicCh <- v
					}
				}()
				for {
					if ctx.Err() != nil {
						return
					}
					start := time.Now()
					err := fn(ctx)
					elapsed := time.Since(start)

					latenciesMu.Lock()
					latencies = append(latencies, elapsed)
					latenciesMu.Unlock()

					total.Add(1)
					if err != nil {
						failures.Add(1)
						if errors.Is(err, resilium.ErrRateLimited) {
							rateLimited.Add(1)
						}
					} else {
						successes.Add(1)
					}
				}
			}()
		}

		wg.Wait()
		close(panicCh)
		for p := range panicCh {
			t.Fatalf("%s: panic in worker: %v", label, p)
		}

		return total.Load(), successes.Load(), failures.Load(), rateLimited.Load(), latencies
	}

	serverHits.Store(0)
	directTotal, directOK, directFail, _, directLat := runLoad("baseline", func(ctx context.Context) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fastServer.URL, nil)
		if err != nil {
			return err
		}
		resp, err := testHTTPClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status %d", resp.StatusCode)
		}
		return nil
	})

	directServerHits := serverHits.Load()

	serverHits.Store(0)
	resiliumTotal, resiliumOK, resiliumFail, resiliumRateLimited, resiliumLat := runLoad("resilium", func(ctx context.Context) error {
		_, err := resilium.Execute(ctx, policy, func(ctx context.Context) (string, error) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, fastServer.URL, nil)
			if err != nil {
				return "", err
			}
			resp, err := testHTTPClient.Do(req)
			if err != nil {
				return "", err
			}
			defer resp.Body.Close()
			_, _ = io.Copy(io.Discard, resp.Body)
			if resp.StatusCode != http.StatusOK {
				return "", fmt.Errorf("status %d", resp.StatusCode)
			}
			return "ok", nil
		})
		return err
	})

	logStats := func(label string, total, ok, fail int64, latencies []time.Duration) {
		p50 := percentile(latencies, 0.50)
		p95 := percentile(latencies, 0.95)
		p99 := percentile(latencies, 0.99)
		rps := float64(total) / loadDuration.Seconds()
		t.Logf("%s: execute_calls=%d success=%d failure=%d loop_rps=%.0f p50=%s p95=%s p99=%s (latency of Execute/HTTP attempt, not server-only)",
			label, total, ok, fail, rps, p50, p95, p99)
	}

	logStats("baseline", directTotal, directOK, directFail, directLat)
	t.Logf("baseline: server_hits=%d (%.1f%% of execute calls reached httptest server)",
		directServerHits, 100*float64(directServerHits)/float64(directTotal))

	resiliumServerHits := serverHits.Load()
	logStats("resilium", resiliumTotal, resiliumOK, resiliumFail, resiliumLat)
	t.Logf("resilium: server_hits=%d (%.2f%% of execute calls reached server); rate_limited=%d (%.1f%% of execute calls, fast-path rejections)",
		resiliumServerHits,
		100*float64(resiliumServerHits)/float64(resiliumTotal),
		resiliumRateLimited,
		100*float64(resiliumRateLimited)/float64(resiliumTotal))

	if resiliumTotal == 0 {
		t.Fatal("resilium load produced zero requests")
	}
	if resiliumOK == 0 {
		t.Fatal("resilium load produced zero successes")
	}

	directP99 := percentile(directLat, 0.99)
	resiliumP99 := percentile(resiliumLat, 0.99)
	if directP99 > 0 && resiliumP99 > directP99*time.Duration(p99Multiplier) {
		t.Fatalf("resilium p99 %s > %dx baseline p99 %s (possible perf cliff)",
			resiliumP99, p99Multiplier, directP99)
	}

	overheadPct := (float64(resiliumP99)/float64(directP99) - 1) * 100
	if directP99 > 0 {
		t.Logf("informational loop p99 comparison: %.1f%% vs baseline (NOT end-to-end server latency — "+
			"resilium p99 dominated by ErrRateLimited fast-path when %.1f%% of calls never reach server)",
			overheadPct, 100*float64(resiliumTotal-resiliumServerHits)/float64(resiliumTotal))
	}
}

func percentile(latencies []time.Duration, p float64) time.Duration {
	if len(latencies) == 0 {
		return 0
	}
	sorted := append([]time.Duration(nil), latencies...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	idx := int(float64(len(sorted)-1) * p)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
