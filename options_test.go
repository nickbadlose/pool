package pool

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWithPoolTimeout(t *testing.T) {
	cases := []struct {
		name     string
		timeout  time.Duration
		expected time.Duration
	}{
		{
			name:     "it should set the timeout to the given value",
			timeout:  1 * time.Second,
			expected: 1 * time.Second,
		},
		{
			name:     "it should limit the timeout to the max value",
			timeout:  1000 * time.Hour,
			expected: time.Duration(maxPoolTimeout),
		},
		{
			name:     "it should limit the lower boundary to 0",
			timeout:  -1 * time.Hour,
			expected: time.Duration(0),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config{}
			opt := WithPoolTimeout(tc.timeout)
			opt.apply(cfg)
			require.NotNil(t, cfg.poolTimeout)
			require.Equal(t, tc.expected, *cfg.poolTimeout)
		})
	}
}

func TestWithWorkTimeout(t *testing.T) {
	cases := []struct {
		name     string
		timeout  time.Duration
		expected time.Duration
	}{
		{
			name:     "it should set the timeout to the given value",
			timeout:  1 * time.Second,
			expected: 1 * time.Second,
		},
		{
			name:     "it should limit the timeout to the max value",
			timeout:  1000 * time.Hour,
			expected: time.Duration(maxWorkTimeout),
		},
		{
			name:     "it should limit the lower boundary to 0",
			timeout:  -1 * time.Hour,
			expected: time.Duration(0),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config{}
			opt := WithWorkTimeout(tc.timeout)
			opt.apply(cfg)
			require.Equal(t, tc.expected, cfg.workTimeout)
		})
	}
}

func TestWithWorkers(t *testing.T) {
	cases := []struct {
		name     string
		workers  int
		expected int
	}{
		{
			name:     "it should set the timeout to the given value",
			workers:  10,
			expected: 10,
		},
		{
			name:     "it should limit the workers to the max value",
			workers:  1000,
			expected: maxWorkers,
		},
		{
			name:     "it should limit the lower boundary to 1",
			workers:  -10,
			expected: 1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config{}
			opt := WithWorkers(tc.workers)
			opt.apply(cfg)
			require.Equal(t, tc.expected, cfg.workers)
		})
	}
}
