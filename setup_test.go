package pool

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"math/rand"
	"sync"
	"time"
)

const jobs = 100

var btx = context.Background()

type testWorker struct {
	i int
}

func (t *testWorker) Work(context.Context) any {
	// simulate work being done, makes tests reliable
	time.Sleep(1 * time.Millisecond)
	return t.i
}

func testWorkerHandler(w any) (Worker, error) {
	i := w.(int) + 1
	return &testWorker{i: i}, nil
}

type errWorker struct {
	i int
}

type errWorkerRes struct {
	i   int
	err error
}

func (t *errWorker) Work(context.Context) any {
	// simulate work being done, makes tests reliable
	time.Sleep(1 * time.Millisecond)
	if t.i == 2 {
		return &errWorkerRes{i: t.i, err: errors.New("work error")}
	}
	return &errWorkerRes{i: t.i, err: nil}
}

func errWorkerHandler(w any) (Worker, error) {
	ew, ok := w.(*errWorkerRes)
	if !ok {
		return nil, errors.New("not a errWorkerRes")
	}
	if ew.err != nil {
		return nil, ew.err
	}
	return &errWorker{i: ew.i + 1}, nil
}

type heavyWorker map[string]string

type hwRes struct {
	err  error
	data []byte
}

func (h heavyWorker) Work(_ context.Context) any {
	data, err := json.Marshal(h)
	return &hwRes{err: err, data: data}
}

func heavyWorkerHandler(w any) (Worker, error) {
	got, ok := w.(*hwRes)
	if !ok {
		return nil, errors.New("not a hwRes")
	}
	if got.err != nil {
		return nil, got.err
	}

	hwk := heavyWorker{}
	err := json.Unmarshal(got.data, &hwk)
	if got.err != nil {
		return nil, err
	}
	return hwk, nil
}

// helper generates a heavy worker
func genHeavyWorker() heavyWorker {
	// n represents the number of items, the length of the key and values in the map.
	n := 100
	h := make(map[string]string, n)
	for i := 0; i < n; i++ {
		h[randSeq(n)] = randSeq(n)
	}
	return h
}

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// helper gets either a buffered or unbuffered dispatcher.
func getDispatcher(ctx context.Context, wp *WorkerPool, jobs int, buffered bool) *Dispatcher {
	d := wp.Start(ctx)
	if buffered {
		d = wp.Start(ctx, jobs)
	}
	return d
}

// typical dispatcher pipeline for test purposes.
//
// We can re use this function to run the pipeline through different scenarios, including fuzzing.
func testDispatcherPipeline(
	jobs int,
	buffered bool,
	initialWorker Worker,
	handler func(any) (Worker, error),
	opts ...Option,
) ([]Worker, error) {
	ctx := context.Background()
	wp := NewWorkerPool(opts...)

	d := getDispatcher(ctx, wp, jobs, buffered)
	go func() {
		defer d.Done()
		for i := 0; i < jobs; i++ {
			err := d.Dispatch(initialWorker)
			if err != nil {
				return
			}
		}
	}()

	d2 := getDispatcher(ctx, wp, jobs, buffered)
	go func() {
		defer d2.Done()
		for r := range d.Receive() {
			w, err := handler(r)
			if err != nil {
				d.SetErr(err)
				return
			}
			err = d2.Dispatch(w)
			if err != nil {
				return
			}
		}
	}()

	d3 := getDispatcher(ctx, wp, jobs, buffered)
	go func() {
		defer d3.Done()
		for r := range d2.Receive() {
			w, err := handler(r)
			if err != nil {
				d.SetErr(err)
				return
			}
			err = d3.Dispatch(w)
			if err != nil {
				return
			}
		}
	}()

	res := make([]Worker, 0)
	for r := range d3.Receive() {
		w, err := handler(r)
		if err != nil {
			d.SetErr(err)
			break
		}

		res = append(res, w)
	}

	return res, wp.Err(d, d2, d3)
}

// TODO clean up steps in here, we don't need so many, some receivers / dispatchers can be omitted for the same result

// for benchmarking a standard worker pool setup.
func testGenericPipeline(jobs, workers int, initialWorker Worker, handler func(any) (Worker, error)) error {
	ctx := context.Background()
	d1 := make(chan Worker)
	rcv1 := make(chan any)

	go func() {
		defer close(d1)
		for i := 0; i < jobs; i++ {
			d1 <- initialWorker
		}
	}()

	go func() {
		wg := sync.WaitGroup{}
		defer func() {
			wg.Wait()
			close(rcv1)
		}()

		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for w := range d1 {
					rcv1 <- w.Work(ctx)
				}
			}()
		}
	}()

	d2 := make(chan Worker)
	rcv2 := make(chan any)
	go func() {
		defer close(d2)
		for r := range rcv1 {
			w, err := handler(r)
			if err != nil {
				log.Fatalf("receive response: %v", err)
			}

			d2 <- w
		}
	}()

	go func() {
		wg := sync.WaitGroup{}
		defer func() {
			wg.Wait()
			close(rcv2)
		}()

		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for w := range d2 {
					rcv2 <- w.Work(ctx)
				}
			}()
		}
	}()

	d3 := make(chan Worker)
	rcv3 := make(chan any)
	go func() {
		defer close(d3)
		for r := range rcv2 {
			w, err := handler(r)
			if err != nil {
				log.Fatalf("receive response: %v", err)
			}

			d3 <- w
		}
	}()

	go func() {
		wg := sync.WaitGroup{}
		defer func() {
			wg.Wait()
			close(rcv3)
		}()

		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for w := range d3 {
					rcv3 <- w.Work(ctx)
				}
			}()
		}
	}()

	res := make([]Worker, 0)
	for r := range rcv3 {
		w, err := handler(r)
		if err != nil {
			log.Fatalf("receive response: %v", err)
		}

		res = append(res, w)
	}

	return nil
}
