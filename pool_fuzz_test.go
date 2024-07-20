package pool

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func FuzzPool(f *testing.F) {
	if testing.Short() {
		f.Skip("skipping fuzz tests in short mode.")
	}

	f.Add(12, 25, 31, 10, 30, 32, 321, false)
	f.Add(654224, 245, 342, 1024, 10043, 100, 5, true)
	f.Add(1, 1, 1, 1, 1, 1, 1, true)
	f.Add(56, 4, 32, 100, 100, 1, 1000, true)
	f.Fuzz(func(t *testing.T, stage1, stage2, stage3, workTimeout, poolTimeout, workers, jobs int, withJobs bool) {
		var buf bytes.Buffer
		buf.WriteString(fmt.Sprintf(
			"\n%d,\n%d,\n%d,\n%d,\n%d,\n%d,\n%d,\n%t,\n",
			stage1,
			stage2,
			stage3,
			workTimeout,
			poolTimeout,
			workers,
			jobs,
			withJobs,
		))

		// log input so we can replicate any failures easily.
		t.Logf("input: %s", buf.String())

		// jobs < 0 is an invalid assertion value.
		if jobs < 0 {
			t.Skip("jobs < 0")
		}

		// helper gets either a buffered or unbuffered dispatcher depending on withJobs.
		getDispatcher := func(ctx context.Context, wp *WorkerPool, jobs int, withJobs bool) *Dispatcher {
			d := wp.Start(ctx)
			if withJobs {
				d = wp.Start(ctx, jobs)
			}
			return d
		}

		ctx := context.Background()

		wp := New(
			WithWorkers(workers),
			WithWorkTimeout(time.Duration(workTimeout)*time.Second),
			WithPoolTimeout(time.Duration(poolTimeout)*time.Second),
		)

		d := getDispatcher(ctx, wp, jobs, withJobs)
		go func() {
			defer func() {
				d.Done()
			}()
			for i := 0; i < jobs; i++ {
				err := d.Dispatch(&testWorker{stage1})
				if err != nil {
					t.Logf("%v", err)
					require.ErrorIs(t, err, ErrDispatcherClosed)
					return
				}
			}
		}()

		d2 := getDispatcher(ctx, wp, jobs, withJobs)
		go func() {
			defer func() {
				d2.Done()
			}()
			for r := range d.Receive() {
				i := r.(int) + stage2
				err := d2.Dispatch(&testWorker{i: i})
				if err != nil {
					t.Logf("%v", err)
					require.ErrorIs(t, err, ErrDispatcherClosed)
					return
				}
			}
		}()

		d3 := getDispatcher(ctx, wp, jobs, withJobs)
		go func() {
			defer func() {
				d3.Done()
			}()
			for r := range d2.Receive() {
				i := r.(int) + stage3
				err := d3.Dispatch(&testWorker{i: i})
				if err != nil {
					t.Logf("%v", err)
					require.ErrorIs(t, err, ErrDispatcherClosed)
					return
				}
			}
		}()

		res := make([]int, 0)
		for r := range d3.Receive() {
			res = append(res, r.(int))
		}

		// if the timeout provided is low, our work / pool context will be cancelled before the pipeline finishes,
		// this is the expected behaviour, so we don't fail our tests.
		err := wp.Err(d, d2, d3)
		if err != nil {
			require.Error(t, err)
			require.ErrorIs(t, err, context.DeadlineExceeded)
			require.True(t, timeoutIsLow(workTimeout, poolTimeout))
			require.LessOrEqual(t, len(res), jobs)
			return
		}

		require.NoError(t, err)
		// require all jobs were completed
		require.Equal(t, jobs, len(res))
		// require each job went through each stage of pipeline
		for _, got := range res {
			require.Equal(t, stage1+stage2+stage3, got)
		}
	})
}

// accepted values for timeout to be hit in tests, causing context to cancel.
func timeoutIsLow(workTimeout, poolTimeout int) bool {
	return workTimeout < int(100*time.Millisecond) || poolTimeout < int(1*time.Second)
}
