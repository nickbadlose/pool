package pool

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewWorkerPool(t *testing.T) {
	t.Run("it should set default options", func(t *testing.T) {
		wp := NewWorkerPool()
		require.NotNil(t, wp)
		require.Equal(t, defaultWorkers, wp.workers())
		require.Equal(t, defaultWorkTimeout, wp.workTimeout())
		require.Nil(t, wp.poolTimeout())
	})

	t.Run("it should set provided options", func(t *testing.T) {
		wp := NewWorkerPool(WithWorkers(10), WithPoolTimeout(10*time.Second), WithWorkTimeout(10*time.Second))
		require.NotNil(t, wp)
		require.Equal(t, 10, wp.workers())
		require.Equal(t, 10*time.Second, wp.workTimeout())
		require.NotNil(t, wp.poolTimeout())
		require.Equal(t, 10*time.Second, *wp.poolTimeout())
	})

	t.Run("Start should return a configured Dispatcher", func(t *testing.T) {
		wp := NewWorkerPool()
		require.NotNil(t, wp)
		d := wp.Start(context.Background())
		require.NotNil(t, d)
		require.NoError(t, wp.Err(d))
	})
}

func TestWorkerPoolEmpty(t *testing.T) {
	t.Run("it should not panic if config is nil", func(t *testing.T) {
		wp := &WorkerPool{}
		require.Equal(t, 1, wp.workers())
		require.Equal(t, time.Duration(0), wp.workTimeout())
		require.Nil(t, wp.poolTimeout())

		d := wp.Start(context.Background())
		require.NotNil(t, d)
		require.NoError(t, wp.Err(d))
	})
}
