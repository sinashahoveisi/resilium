// Command basic demonstrates using resilium's retry, timeout, and rate
// limiting policies to call a flaky operation. This is the canonical
// Quick Start example — keep it in sync with README.md.
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/sinashahoveisi/resilium"
	"github.com/sinashahoveisi/resilium/retry"
)

func main() {
	policy := resilium.New(
		resilium.WithRetry(retry.Config{
			MaxAttempts: 3,
			Backoff:     retry.ExponentialBackoff(100*time.Millisecond, 1*time.Second),
		}),
		resilium.WithTimeout(2*time.Second),
		resilium.WithRateLimit(10, 10),
	)

	result, err := resilium.Execute(context.Background(), policy, callFlakyService)
	if err != nil {
		fmt.Println("failed after retries:", err)
		return
	}
	fmt.Println("got:", result)
}

func callFlakyService(ctx context.Context) (string, error) {
	// Replace with a real network call. This is a placeholder for the
	// example so it compiles and runs standalone.
	return "hello from resilium", nil
}
