package pool

import "time"

const (
	maxWorkers     = 50
	minWorkers     = 1
	maxWorkTimeout = int(10 * time.Minute)
	maxPoolTimeout = int(30 * time.Minute)
)

// Option interface allows us to safely apply configuration to a workerPool
type Option interface {
	apply(*config)
}

// config for our workerPool configuration.
type config struct {
	// number of workers.
	workers int
	// timeout to process a single job.
	workTimeout time.Duration
	// timeout for all jobs to be processed.
	poolTimeout *time.Duration
}

type optionFunc struct {
	f func(*config)
}

// apply implements Option, simply calls s.f
func (s *optionFunc) apply(cfg *config) {
	s.f(cfg)
}

// newOptionFunc generates a Option from a function.
func newOptionFunc(fn func(*config)) Option {
	return &optionFunc{f: fn}
}

// WithWorkers allows us to override the number of workers the jobs should be buffered to.
func WithWorkers(n int) Option {
	return newOptionFunc(func(cfg *config) {
		cfg.workers = constrictToRange(n, minWorkers, maxWorkers)
	})
}

// WithWorkTimeout allows us to override the timeout on each singular job being completed.
func WithWorkTimeout(duration time.Duration) Option {
	return newOptionFunc(func(cfg *config) {
		cfg.workTimeout = time.Duration(constrictToRange(int(duration), 0, maxWorkTimeout))
	})
}

// WithPoolTimeout allows us to override the timeout of the worker pool, this is for all jobs to be completed.
func WithPoolTimeout(duration time.Duration) Option {
	return newOptionFunc(func(cfg *config) {
		tOut := time.Duration(constrictToRange(int(duration), 0, maxPoolTimeout))
		cfg.poolTimeout = &tOut
	})
}

// constrictToRange constricts n between the bounds of l & u.
func constrictToRange(n, l, u int) int {
	switch {
	case n < l:
		return l
	case n > u:
		return u
	}

	return n
}
