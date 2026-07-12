package integration_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"time"
)

var testHTTPClient = &http.Client{
	Timeout: 5 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        4,
		MaxIdleConnsPerHost: 2,
		IdleConnTimeout:     50 * time.Millisecond,
	},
}

func closeTestHTTPIdle() {
	if tr, ok := testHTTPClient.Transport.(*http.Transport); ok {
		tr.CloseIdleConnections()
	}
}

type serverMode int32

const (
	modeOK serverMode = iota
	modeStatus500
	modeFailFirstN
	modeSlow
	modeChaos
)

type switchableServer struct {
	srv       *httptest.Server
	hits      atomic.Int64
	mode      atomic.Int32
	failUntil atomic.Int32
	slowFor   atomic.Int64
}

func newSwitchableServer() *switchableServer {
	s := &switchableServer{}
	s.slowFor.Store(int64(500 * time.Millisecond))
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit := s.hits.Add(1)

		switch r.URL.Path {
		case "/fast":
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "ok")
			return
		case "/slow":
			time.Sleep(time.Duration(s.slowFor.Load()))
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "ok")
			return
		}

		switch serverMode(s.mode.Load()) {
		case modeStatus500:
			w.WriteHeader(http.StatusInternalServerError)
		case modeFailFirstN:
			if int32(hit) <= s.failUntil.Load() {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "ok")
		case modeSlow:
			time.Sleep(time.Duration(s.slowFor.Load()))
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "ok")
		case modeChaos:
			switch hit % 7 {
			case 0:
				time.Sleep(80 * time.Millisecond)
			case 1, 2:
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "ok")
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "ok")
		}
	}))
	return s
}

func (s *switchableServer) Close() {
	s.srv.Close()
}

func (s *switchableServer) URL(path string) string {
	return s.srv.URL + path
}

func (s *switchableServer) HitCount() int64 {
	return s.hits.Load()
}

func (s *switchableServer) SetMode(m serverMode) {
	s.mode.Store(int32(m))
}

func (s *switchableServer) SetFailUntil(n int32) {
	s.failUntil.Store(n)
}

func (s *switchableServer) SetSlowDelay(d time.Duration) {
	s.slowFor.Store(int64(d))
}

func httpGet(ctx context.Context, url string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := testHTTPClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}

func requireHTTPOK(ctx context.Context, url string) error {
	code, err := httpGet(ctx, url)
	if err != nil {
		return err
	}
	if code != http.StatusOK {
		return fmt.Errorf("HTTP %d", code)
	}
	return nil
}

func errFromHTTPStatus(code int) error {
	if code == http.StatusOK {
		return nil
	}
	return fmt.Errorf("unexpected status %d", code)
}
