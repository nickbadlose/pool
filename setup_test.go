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

func (t *testWorker) Work(context.Context) interface{} {
	// simulate work being done, makes tests reliable
	time.Sleep(1 * time.Millisecond)
	return t.i
}

type heavyWorker map[string]string

type hwRes struct {
	err  error
	data []byte
}

func (h heavyWorker) Work(_ context.Context) interface{} {
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

func testWorkerHandler(w any) (Worker, error) {
	i := w.(int) + 1
	return &testWorker{i: i}, nil
}

func genHeavyWorker() heavyWorker {
	h := make(map[string]string, 100)
	for i := 0; i < 100; i++ {
		h[randSeq(30)] = randSeq(30)
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

// typical dispatcher pipeline for test purposes.
func testDispatcherPipeline(jobs, workers int, buffered bool, initialWorker Worker, handler func(any) (Worker, error)) ([]Worker, error) {
	wp := NewWorkerPool(WithWorkers(workers))

	d := getDispatcher(btx, wp, jobs, buffered)
	go func() {
		defer d.Done()
		for i := 0; i < jobs; i++ {
			err := d.Dispatch(initialWorker)
			if err != nil {
				return
			}
		}
	}()

	d2 := getDispatcher(btx, wp, jobs, buffered)
	go func() {
		defer d2.Done()
		for r := range d.Receive() {
			w, err := handler(r)
			if err != nil {
				log.Fatalf("receive response: %v", err)
			}
			err = d2.Dispatch(w)
			if err != nil {
				return
			}
		}
	}()

	d3 := getDispatcher(btx, wp, jobs, buffered)
	go func() {
		defer d3.Done()
		for r := range d2.Receive() {
			w, err := handler(r)
			if err != nil {
				log.Fatalf("receive response: %v", err)
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
			log.Fatalf("receive response: %v", err)
		}

		res = append(res, w)
	}

	return res, wp.Err(d, d2, d3)
}

// for benchmarking a standard worker pool setup.
func testCustomPipeline(jobs, workers int, initialWorker Worker, handler func(any) (Worker, error)) error {
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
					rcv1 <- w.Work(btx)
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
					rcv2 <- w.Work(btx)
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
					rcv3 <- w.Work(btx)
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
