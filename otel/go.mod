module github.com/sinashahoveisi/resilium/otel

go 1.22

require (
	// Regenerate this pin after pushing root-module changes: cd otel && go get github.com/sinashahoveisi/resilium@main && go mod tidy
	github.com/sinashahoveisi/resilium v0.0.0-20260711195640-136335345b6a
	go.opentelemetry.io/otel v1.32.0
	go.opentelemetry.io/otel/metric v1.32.0
	go.opentelemetry.io/otel/sdk/metric v1.32.0
)

require (
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	go.opentelemetry.io/otel/sdk v1.32.0 // indirect
	go.opentelemetry.io/otel/trace v1.32.0 // indirect
	golang.org/x/sys v0.27.0 // indirect
)
