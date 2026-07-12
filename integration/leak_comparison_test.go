//go:build leakcompare

// Opt-in goroutine leak comparison diagnostic (July 2026).
//
// Added during pre-v1.0 integration test review to prove the original +10
// goroutine delta in leak testing came from http.DefaultClient idle
// connections and baseline methodology — not a resilium goroutine leak.
//
// Not compiled or run by default CI or `go test ./...`. Run manually:
//
//	go test -tags leakcompare -race -count=1 -run TestLeakComparison -v ./integration/
//
// Also skipped under -short even when the leakcompare tag is set.
package integration_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"testing"
	"time"

	"github.com/sinashahoveisi/resilium"
	"github.com/sinashahoveisi/resilium/circuitbreaker"
	"github.com/sinashahoveisi/resilium/retry"
)

const leakComparisonIterations = 3000

type leakHTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

func skipLeakComparisonShort(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("leak comparison diagnostic: omit -short when running with -tags leakcompare")
	}
}

func httpGetWithClient(ctx context.Context, client leakHTTPClient, url string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}

func requireHTTPOKWithClient(ctx context.Context, client leakHTTPClient, url string) error {
	code, err := httpGetWithClient(ctx, client, url)
	if err != nil {
		return err
	}
	if code != http.StatusOK {
		return fmt.Errorf("HTTP %d", code)
	}
	return nil
}

func leakTestPolicies() (
	success *resilium.Policy,
	timeout *resilium.Policy,
	retryExhaust *resilium.Policy,
	cb *resilium.Policy,
) {
	success = resilium.New(
		resilium.WithTimeout(100*time.Millisecond),
		resilium.WithRetry(retry.Config{
			MaxAttempts: 3,
			Backoff:     retry.FixedBackoff(time.Millisecond),
		}),
	)
	timeout = resilium.New(resilium.WithTimeout(15 * time.Millisecond))
	retryExhaust = resilium.New(
		resilium.WithRetry(retry.Config{
			MaxAttempts: 3,
			Backoff:     retry.FixedBackoff(time.Millisecond),
		}),
	)
	cb = resilium.New(
		resilium.WithRetry(retry.Config{
			MaxAttempts: 5,
			Backoff:     retry.FixedBackoff(time.Millisecond),
		}),
		resilium.WithCircuitBreaker(circuitbreaker.Config{
			FailureThreshold: 0.5,
			MinRequests:      2,
			OpenDuration:     50 * time.Millisecond,
		}),
	)
	return success, timeout, retryExhaust, cb
}

func runDirectLeakLoop(ctx context.Context, srv *switchableServer, client leakHTTPClient, iterations int) {
	for i := 0; i < iterations; i++ {
		switch i % 4 {
		case 0:
			srv.SetMode(modeOK)
			_ = requireHTTPOKWithClient(ctx, client, srv.URL("/"))
		case 1:
			srv.SetMode(modeStatus500)
			code, err := httpGetWithClient(ctx, client, srv.URL("/"))
			if err == nil {
				_ = errFromHTTPStatus(code)
			}
		case 2:
			srv.SetSlowDelay(200 * time.Millisecond)
			code, err := httpGetWithClient(ctx, client, srv.URL("/slow"))
			if err == nil {
				_ = errFromHTTPStatus(code)
			}
		case 3:
			srv.SetMode(modeStatus500)
			code, err := httpGetWithClient(ctx, client, srv.URL("/"))
			if err == nil {
				_ = errFromHTTPStatus(code)
			}
		}
	}
}

func runResiliumLeakLoop(
	ctx context.Context,
	srv *switchableServer,
	client leakHTTPClient,
	success, timeout, retryExhaust, cb *resilium.Policy,
	iterations int,
) {
	for i := 0; i < iterations; i++ {
		switch i % 4 {
		case 0:
			srv.SetMode(modeOK)
			_, _ = resilium.Execute(ctx, success, func(ctx context.Context) (struct{}, error) {
				if err := requireHTTPOKWithClient(ctx, client, srv.URL("/")); err != nil {
					return struct{}{}, err
				}
				return struct{}{}, nil
			})
		case 1:
			srv.SetMode(modeStatus500)
			_, _ = resilium.Execute(ctx, retryExhaust, func(ctx context.Context) (struct{}, error) {
				code, err := httpGetWithClient(ctx, client, srv.URL("/"))
				if err != nil {
					return struct{}{}, err
				}
				return struct{}{}, errFromHTTPStatus(code)
			})
		case 2:
			srv.SetSlowDelay(200 * time.Millisecond)
			_, _ = resilium.Execute(ctx, timeout, func(ctx context.Context) (struct{}, error) {
				code, err := httpGetWithClient(ctx, client, srv.URL("/slow"))
				if err != nil {
					return struct{}{}, err
				}
				return struct{}{}, errFromHTTPStatus(code)
			})
		case 3:
			srv.SetMode(modeStatus500)
			_, _ = resilium.Execute(ctx, cb, func(ctx context.Context) (struct{}, error) {
				code, err := httpGetWithClient(ctx, client, srv.URL("/"))
				if err != nil {
					return struct{}{}, err
				}
				return struct{}{}, errFromHTTPStatus(code)
			})
		}
	}
}

func runLeakComparison(
	t *testing.T,
	label string,
	baselineBeforeServer bool,
	closeServerBeforeMeasure bool,
	closeIdleBeforeMeasure bool,
	useDefaultClient bool,
	useResilium bool,
) {
	t.Helper()
	skipLeakComparisonShort(t)

	var baseline int
	if baselineBeforeServer {
		baseline = countGoroutines()
	}

	srv := newSwitchableServer()
	if !baselineBeforeServer {
		baseline = countGoroutines()
	}

	client := leakHTTPClient(testHTTPClient)
	if useDefaultClient {
		client = http.DefaultClient
	}

	ctx := context.Background()
	success, timeout, retryExhaust, cb := leakTestPolicies()

	if useResilium {
		runResiliumLeakLoop(ctx, srv, client, success, timeout, retryExhaust, cb, leakComparisonIterations)
	} else {
		runDirectLeakLoop(ctx, srv, client, leakComparisonIterations)
	}

	if closeServerBeforeMeasure {
		srv.Close()
	}
	if closeIdleBeforeMeasure {
		if useDefaultClient {
			closeDefaultHTTPIdle()
		} else {
			closeTestHTTPIdle()
		}
	}
	settleGoroutines()
	after := runtime.NumGoroutine()
	delta := after - baseline

	baselineWhen := "after_server"
	if baselineBeforeServer {
		baselineWhen = "before_server"
	}
	cleanup := fmt.Sprintf("close_server=%v close_idle=%v", closeServerBeforeMeasure, closeIdleBeforeMeasure)

	t.Logf("%s: baseline=%d (%s) after=%d delta=%+d [%s] client=%s mode=%s",
		label, baseline, baselineWhen, after, delta, cleanup,
		map[bool]string{true: "DefaultClient", false: "testHTTPClient"}[useDefaultClient],
		map[bool]string{true: "resilium", false: "direct"}[useResilium])
}

func closeDefaultHTTPIdle() {
	if tr, ok := http.DefaultTransport.(*http.Transport); ok {
		tr.CloseIdleConnections()
	}
}

func TestLeakComparisonA_DirectDefaultClient(t *testing.T) {
	runLeakComparison(t, "A_direct_default", false, false, false, true, false)
}

func TestLeakComparisonB_ResiliumDefaultClient(t *testing.T) {
	runLeakComparison(t, "B_resilium_default", false, false, false, true, true)
}

func TestLeakComparisonOriginal_DirectDefaultClient(t *testing.T) {
	runLeakComparison(t, "original_direct_default", true, false, false, true, false)
}

func TestLeakComparisonOriginal_ResiliumDefaultClient(t *testing.T) {
	runLeakComparison(t, "original_resilium_default", true, false, false, true, true)
}

func TestLeakComparisonFixed_ResiliumTestClient(t *testing.T) {
	runLeakComparison(t, "fixed_resilium_testclient", false, true, true, false, true)
}
