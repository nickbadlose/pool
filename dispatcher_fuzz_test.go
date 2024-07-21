package pool

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func FuzzDispatcherPipeline(f *testing.F) {
	if testing.Short() {
		f.Skip("skipping fuzz tests in short mode.")
	}

	f.Add(10, 30, 32, 321, false)
	f.Add(1024, 10043, 100, 5, true)
	f.Add(1, 1, 1, 1, true)
	f.Add(100, 100, 1, 1000, true)
	f.Fuzz(func(t *testing.T, workTimeout, poolTimeout, workers, jobs int, buffered bool) {
		var buf bytes.Buffer
		buf.WriteString(fmt.Sprintf(
			"\n%d,\n%d,\n%d,\n%d,\n%t,\n",
			workTimeout,
			poolTimeout,
			workers,
			jobs,
			buffered,
		))

		// log input so we can replicate any failures easily.
		t.Logf("input: %s", buf.String())

		// jobs < 0 is an invalid assertion value.
		if jobs < 0 {
			t.Skip("jobs < 0")
		}

		res, err := testDispatcherPipeline(
			jobs,
			buffered,
			&testWorker{1},
			testWorkerHandler,
			WithWorkers(workers),
			WithWorkTimeout(time.Duration(workTimeout)*time.Second),
			WithPoolTimeout(time.Duration(poolTimeout)*time.Second),
		)
		// if the timeout provided is low, our work / pool context will be cancelled before the pipeline finishes,
		// this is the expected behaviour, so we don't fail our tests.
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
			tw, ok := got.(*testWorker)
			require.True(t, ok)
			require.Equal(t, 4, tw.i)
		}
	})
}

// accepted values for timeout to be hit in tests, causing context to cancel.
func timeoutIsLow(workTimeout, poolTimeout int) bool {
	return workTimeout < int(100*time.Millisecond) || poolTimeout < int(1*time.Second)
}
