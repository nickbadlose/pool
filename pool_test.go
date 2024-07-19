package pool

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TODO clean up tests, testing options should be done separately etc.

// TODO benchmarks with more workers and no concurrency etc. Add in 100 * time.MilliSecond to simulate http requests etc. Do a mock server that hangs for 100 ms before responding with a heavy response?
// TODO benchmarks also for normal concurrency pipeline vs one configured by this? Do one that is set up by configuring all go routines first and using semaphore to limit number of active ones?
// TODO README, semantic release, linting, CICD
// TODO fuzz testing for nil pointers etc.

const jobs = 100

var btx = context.Background()

type testWorker struct {
	i int
}

func (t *testWorker) Work(context.Context) interface{} {
	// simulate work being done, makes tests reliable
	time.Sleep(1 * time.Millisecond)
	return t.i
}

func TestWorkerPool(t *testing.T) {
	// number of workers should be set to 1 when negative number passed
	wp := New(
		WithWorkers(-3),
		WithPoolTimeout(1*time.Minute),
		WithWorkTimeout(1*time.Minute),
	)

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
	require.Equal(t, jobs, len(res))
}

func TestContextCancelled(t *testing.T) {
	wp := New(
		WithWorkers(3),
		WithPoolTimeout(10*time.Minute),
		WithWorkTimeout(1*time.Minute),
	)

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
	// when ctx was cancelled we should receive an error once the dispatch channel is closed due to this.
	require.Error(t, err)

	// we should not panic if context is cancelled, we should shut down gracefully.
	d.Done()

	// any jobs after context is cancelled should not be received, so len should be < than the number of jobs.
	require.Less(t, len(res), jobs)
	// at least one job is processed before context is cancelled, so len should be > 0.
	require.Greater(t, len(res), 0)
}

func TestWorkerChanClosed(t *testing.T) {
	wp := New(
		WithWorkers(3),
		WithPoolTimeout(10*time.Minute),
		WithWorkTimeout(1*time.Minute),
	)

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

func TestPipeline(t *testing.T) {
	// TODO no need for go funcs for first stage of dispatching when you know the number of jobs
	//  Docs should always state to use go funcs at all times anyway and send number of jobs if known for performance reasons,
	//  Check with benchmarks against two types

	// worker and timeouts will be constricted to range when over the limit.
	wp := New(
		WithWorkers(100),
		WithPoolTimeout(100*time.Minute),
		WithWorkTimeout(100*time.Minute),
	)

	d := wp.Start(btx, jobs)
	go func() {
		defer d.Done()
		for i := 0; i < jobs; i++ {
			// set i = 1 for each worker
			err := d.Dispatch(&testWorker{1})
			require.NoError(t, err)
		}
	}()

	d2 := wp.Start(btx, jobs)
	go func() {
		defer d2.Done()
		for r := range d.Receive() {
			// increment i by 1 so any workers should have i = 2 after this stage
			i := r.(int) + 1
			err := d2.Dispatch(&testWorker{i: i})
			require.NoError(t, err)
		}
	}()

	d3 := wp.Start(btx, jobs)
	go func() {
		defer d3.Done()
		for r := range d2.Receive() {
			// increment i by 1 so any workers should have i = 3 after this stage
			i := r.(int) + 1
			err := d3.Dispatch(&testWorker{i: i})
			require.NoError(t, err)
		}
	}()

	res := make([]int, 0)
	for r := range d3.Receive() {
		res = append(res, r.(int))
	}

	// require all jobs were completed
	require.Equal(t, jobs, len(res))
	// require each job went through each stage of pipeline
	for _, n := range res {
		require.Equal(t, 3, n)
	}
}
