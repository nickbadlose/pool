package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"github.com/nickbadlose/pool"
	"log"
	"math/rand"
	"os"
	"runtime/pprof"
)

// TODO
//   - use jobs as size for make slice
//   - use pointers and non pointers for worker type and res type
//   - buffered vs unbuffered, see why unbuffered is slower
var (
	jbs  = flag.Int("jobs", 10000, "number of jobs to perform")
	wrks = flag.Int("workers", 5, "number of workers to handle jobs")
	buf  = flag.Bool("buffered", false, "buffer the channels")

	cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
	memprofile = flag.String("memprofile", "", "write memory profile to this file")
)

func main() {
	flag.Parse()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		err = pprof.StartCPUProfile(f)
		if err != nil {
			log.Fatal(err)
		}
		defer pprof.StopCPUProfile()
	}

	ctx := context.Background()

	jobs := *jbs
	workers := *wrks
	buffered := *buf
	initialWorker := genWorker()

	fmt.Printf("\njobs: %d, workers: %d, buffered: %t\n", jobs, workers, buffered)

	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			log.Fatal(err)
		}
		err = pprof.WriteHeapProfile(f)
		if err != nil {
			log.Fatal(err)
		}
		err = f.Close()
		if err != nil {
			log.Fatal(err)
		}
		return
	}

	wp := pool.NewWorkerPool(pool.WithWorkers(workers))

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

	fin := make([]*worker, 0)
	for r := range d3.Receive() {
		w, err := handler(r)
		if err != nil {
			d.SetErr(err)
			break
		}

		fin = append(fin, w)
	}

	err := wp.Err(d, d2, d3)
	if err != nil {
		log.Fatalf("pipeline failed: %v", err)
	}

	//for _, w := range fin {
	//	//for k, v := range *w {
	//	//	fmt.Println(k, v)
	//	//}
	//}
}

type worker map[string]string

type res struct {
	err  error
	data *worker
}

func (w worker) Work(_ context.Context) any {
	data, err := json.Marshal(w)
	if err != nil {
		return &res{err: err}
	}

	// simulate http request
	//time.Sleep(10 * time.Millisecond)

	nw := &worker{}
	err = json.Unmarshal(data, nw)
	if err != nil {
		return &res{err: err}
	}

	return &res{err: err, data: nw}
}

// helper gets either a buffered or unbuffered dispatcher.
func getDispatcher(ctx context.Context, wp *pool.WorkerPool, jobs int, buffered bool) *pool.Dispatcher {
	d := wp.Start(ctx)
	if buffered {
		d = wp.Start(ctx, jobs)
	}
	return d
}

func handler(w any) (*worker, error) {
	got, ok := w.(*res)
	if !ok {
		return nil, errors.New("not a hwRes")
	}
	if got.err != nil {
		return nil, got.err
	}

	return got.data, nil
}

// helper generates a worker
func genWorker() *worker {
	// n represents the number of items, the length of the key and values in the map.
	n := 100
	h := worker(make(map[string]string, n))
	for i := 0; i < n; i++ {
		h[randSeq(n)] = randSeq(n)
	}
	return &h
}

func randSeq(n int) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
