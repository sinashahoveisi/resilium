package resilium

import "log/slog"

func mergeHooks(base, add Hooks) Hooks {
	return Hooks{
		OnRetry:        chainOnRetry(base.OnRetry, add.OnRetry),
		OnCircuitOpen:  chainOnCircuitOpen(base.OnCircuitOpen, add.OnCircuitOpen),
		OnCircuitClose: chainOnCircuitClose(base.OnCircuitClose, add.OnCircuitClose),
		OnTimeout:      chainOnTimeout(base.OnTimeout, add.OnTimeout),
		OnRateLimited:  chainOnRateLimited(base.OnRateLimited, add.OnRateLimited),
	}
}

func loggerHooks(logger *slog.Logger) Hooks {
	return Hooks{
		OnRetry: func(attempt int, err error) {
			logger.Debug("resilium retry", "attempt", attempt, "error", err)
		},
		OnCircuitOpen: func(name string) {
			logger.Info("resilium circuit breaker open", "name", name)
		},
		OnCircuitClose: func(name string) {
			logger.Info("resilium circuit breaker closed", "name", name)
		},
		OnTimeout: func() {
			logger.Warn("resilium operation timed out")
		},
		OnRateLimited: func() {
			logger.Warn("resilium rate limited")
		},
	}
}

func chainOnRetry(a, b func(attempt int, err error)) func(attempt int, err error) {
	switch {
	case a == nil:
		return b
	case b == nil:
		return a
	default:
		return func(attempt int, err error) {
			a(attempt, err)
			b(attempt, err)
		}
	}
}

func chainOnCircuitOpen(a, b func(name string)) func(name string) {
	switch {
	case a == nil:
		return b
	case b == nil:
		return a
	default:
		return func(name string) {
			a(name)
			b(name)
		}
	}
}

func chainOnCircuitClose(a, b func(name string)) func(name string) {
	switch {
	case a == nil:
		return b
	case b == nil:
		return a
	default:
		return func(name string) {
			a(name)
			b(name)
		}
	}
}

func chainOnTimeout(a, b func()) func() {
	switch {
	case a == nil:
		return b
	case b == nil:
		return a
	default:
		return func() {
			a()
			b()
		}
	}
}

func chainOnRateLimited(a, b func()) func() {
	switch {
	case a == nil:
		return b
	case b == nil:
		return a
	default:
		return func() {
			a()
			b()
		}
	}
}

func (p *Policy) onRetry(attempt int, err error) {
	if p.hooks.OnRetry != nil {
		p.hooks.OnRetry(attempt, err)
	}
}

func (p *Policy) onCircuitOpen(name string) {
	if p.hooks.OnCircuitOpen != nil {
		p.hooks.OnCircuitOpen(name)
	}
}

func (p *Policy) onCircuitClose(name string) {
	if p.hooks.OnCircuitClose != nil {
		p.hooks.OnCircuitClose(name)
	}
}

func (p *Policy) onTimeout() {
	if p.hooks.OnTimeout != nil {
		p.hooks.OnTimeout()
	}
}

func (p *Policy) onRateLimited() {
	if p.hooks.OnRateLimited != nil {
		p.hooks.OnRateLimited()
	}
}
