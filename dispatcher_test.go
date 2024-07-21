package pool

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// TODO clean up tests, testing options should be done separately etc.

// TODO benchmarks with more workers and no concurrency etc. Add in 100 * time.MilliSecond to simulate http requests etc. Do a mock server that hangs for 100 ms before responding with a heavy response?
// TODO benchmarks also for normal concurrency pipeline vs one configured by this? Do one that is set up by configuring all go routines first and using semaphore to limit number of active ones?
// TODO README, semantic release, linting, CICD
// TODO fuzz testing for nil pointers etc.
// Test an empty initialisation of the struct

// TODO no need for go funcs for first stage of dispatching when you know the number of jobs
//  Docs should always state to use go funcs at all times anyway and send number of jobs if known for performance reasons,
//  Check with benchmarks against two types

func TestDispatcherSingle(t *testing.T) {
	wp := NewWorkerPool()

	d := wp.Start(btx, jobs)

	for i := 0; i < jobs; i++ {
		err := d.Dispatch(&testWorker{i})
		require.NoError(t, err)
	}

	d.Done()

	res := make([]int, 0)
	for r := range d.Receive() {
		res = append(res, r.(int))
	}
	require.NoError(t, d.Err())
	require.Equal(t, jobs, len(res))
}

func TestDispatcherContextCancelled(t *testing.T) {
	wp := NewWorkerPool()

	ctx, cancel := context.WithCancel(btx)
	defer cancel()

	d := wp.Start(ctx, jobs)
	for i := 0; i < 10; i++ {
		err := d.Dispatch(&testWorker{i})
		// there should be no dispatch errors initially.
		require.NoError(t, err)
	}

	res := make([]int, 0)
	for r := range d.Receive() {
		// cancel our context and stop workers processing any further work.
		cancel()
		res = append(res, r.(int))
	}

	// attempt to dispatch after context is cancelled.
	err := d.Dispatch(&testWorker{10})
	// when ctx was cancelled we should receive an error when attempting to dispatch as it should be closed.
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)

	// we should not panic if context is cancelled, we should shut down gracefully.
	d.Done()

	// check the error returned from the dispatcher, if any.
	err = d.Err()
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)

	// check the wp error method returns the correct error.
	err = wp.Err(d)
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)

	// any jobs after context is cancelled should not be received, so len should be < than the number of jobs.
	require.Less(t, len(res), jobs)
	// at least one job is processed before context is cancelled, so len should be > 0.
	require.Greater(t, len(res), 0)
}

func TestDispatcherClosed(t *testing.T) {
	wp := NewWorkerPool()

	d := wp.Start(btx, jobs)
	d.Done()
	err := d.Dispatch(&testWorker{1})
	require.Error(t, err)
	require.Equal(t, "dispatcher closed", err.Error())

	res := make([]int, 0)
	for r := range d.Receive() {
		res = append(res, r.(int))
	}

	// no jobs should have been dispatched since the worker channel was immediately closed.
	require.Equal(t, len(res), 0)
}

func TestDispatcherSetErr(t *testing.T) {
	res, err := testDispatcherPipeline(jobs, true, &errWorker{1}, errWorkerHandler)
	require.Error(t, err)
	require.Equal(t, "work error", err.Error())

	// all jobs should exit on the second stage.
	require.Equal(t, 0, len(res))
}

func TestDispatcherPipeline(t *testing.T) {
	res, err := testDispatcherPipeline(jobs, true, &testWorker{1}, testWorkerHandler)
	require.NoError(t, err)

	// require all jobs were completed
	require.Equal(t, jobs, len(res))
	// require each job went through each stage of pipeline
	for _, w := range res {
		tw, ok := w.(*testWorker)
		require.True(t, ok)
		require.Equal(t, 4, tw.i)
	}
}

func TestBufferedWait(t *testing.T) {
	mu := sync.Mutex{}
	wg := sync.WaitGroup{}
	res := make([]int, 0, jobs)

	wp := NewWorkerPool()
	d := wp.Start(btx, jobs)
	wg.Add(1)
	go func() {
		defer wg.Done()
		for r := range d.Receive() {
			mu.Lock()
			res = append(res, r.(int)+1)
			mu.Unlock()
		}
	}()

	for i := 0; i < jobs; i++ {
		err := d.Dispatch(&testWorker{1})
		require.NoError(t, err)
	}
	d.Done()
	wg.Wait()

	require.NoError(t, d.Err())

	// require all jobs were completed
	require.Equal(t, jobs, len(res))
	// require each job went through each stage of pipeline
	for _, i := range res {
		require.Equal(t, 2, i)
	}
}
