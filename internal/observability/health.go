package observability

import (
	"net/http"
	"sync"
	"sync/atomic"
)

// HealthChecker manages server health state.
type HealthChecker struct {
	mu     sync.RWMutex
	ready  atomic.Bool
	live   atomic.Bool
	checks map[string]func() error
}

// NewHealthChecker creates a new health checker.
func NewHealthChecker() *HealthChecker {
	h := &HealthChecker{
		checks: make(map[string]func() error),
	}
	h.live.Store(true)
	h.ready.Store(true)
	return h
}

// RegisterCheck registers a named health check function.
func (h *HealthChecker) RegisterCheck(name string, check func() error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checks[name] = check
}

// SetReady sets the ready state.
func (h *HealthChecker) SetReady(ready bool) {
	h.ready.Store(ready)
}

// SetLive sets the live state.
func (h *HealthChecker) SetLive(live bool) {
	h.live.Store(live)
}

// IsReady returns whether the server is ready.
func (h *HealthChecker) IsReady() bool {
	return h.ready.Load()
}

// IsLive returns whether the server is live.
func (h *HealthChecker) IsLive() bool {
	return h.live.Load()
}

// LiveHandler returns an HTTP handler for liveness checks.
func (h *HealthChecker) LiveHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.IsLive() {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}
}

// ReadyHandler returns an HTTP handler for readiness checks.
func (h *HealthChecker) ReadyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.IsReady() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("Not Ready"))
			return
		}

		// Run all registered checks
		h.mu.RLock()
		checks := make(map[string]func() error, len(h.checks))
		for name, check := range h.checks {
			checks[name] = check
		}
		h.mu.RUnlock()

		for name, check := range checks {
			if err := check(); err != nil {
				w.WriteHeader(http.StatusServiceUnavailable)
				_, _ = w.Write([]byte("Check failed: " + name))
				return
			}
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	}
}

// RegisterHTTPHandlers registers health check handlers on the given mux.
func (h *HealthChecker) RegisterHTTPHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/health/live", h.LiveHandler())
	mux.HandleFunc("/health/ready", h.ReadyHandler())
}
