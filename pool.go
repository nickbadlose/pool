package pool

import (
	"context"
	"errors"
	"sync"
	"time"
)

// TODO
//  Return structs not interfaces?
//  Do we pass in the number of items of work that will need to be completed and pass that into start and use as the buffer for wc?
//  Do we continue to finish any jobs after ctx cancelled? We want to gracefully finish all provided jobs?
//  Can Start take in a []Worker and handle the dispatching? This limits us from being able to chain requests and dispatch another job in the Receive channel
//  Protect against nil throughout
//  rename to Dispatcher, since it is no longer an interface
//  Try seeing if we block a channel by trying to send on it, if cancelling context will still kill it? Or if the reader needs to drain the channel before leaving
//  The above only applies to unbuffered channels, not buffered like our in this.
//  For when we use unbuffered examples, i.e. we don't know the number of items we need to process. We should dispatch items in a go routine, with maybe a DispatchAll method? Take in a []Worker?
//  DispatchAll would have the total number of items to dispatch anyway, so include the func but not useful for unbuffered
//  We could take in variadic number of jobs, in case there are multiple sources for the jobs, if empty use unbuffered channel?
//  Should dispatch take in ctx and use select statement for when dispatching items? Not necessary since we check if channel is closed and return error.
//  Have a Start method and StartWithJobs ? To separate the two options?
//  Close method may need to drain before closing in the case of unbuffered channel. Make sure all items are dispatched and read correctly in success cases.

const (
	defaultWorkers     = 5
	defaultWorkTimeout = 2 * time.Minute
)

// Worker interface allows a user to perform jobs.
type Worker interface {
	Work(context.Context) any
}

// New builds a new WorkerPool.
func New(os ...Option) *WorkerPool {
	cfg := &config{workers: defaultWorkers, workTimeout: defaultWorkTimeout}
	for _, o := range os {
		o.apply(cfg)
	}

	return &WorkerPool{cfg}
}

// WorkerPool allows us to Start worker pools with the given configuration.
type WorkerPool struct {
	*config
}

// Start sets up a worker pool and returns a Dispatcher.
//
// The Dispatcher allows the user to dispatch any jobs they require doing down the worker channel,
// via the Dispatch method. Once all jobs have been dispatched, it is up to the user to signal that the channel
// can be closed, by calling the Done method.
//
// The user can start receiving any completed jobs as early as they like, via the basic Receive method.
func (wp *WorkerPool) Start(ctx context.Context, jobs int) *Dispatcher {
	dr := &Dispatcher{
		ctx:     ctx,
		rcv:     make(chan any, jobs),
		wc:      make(chan Worker, jobs),
		timeout: wp.workTimeout,
		closed:  false,
		wg:      sync.WaitGroup{},
		mu:      sync.RWMutex{},
	}

	wp.do(dr)

	return dr
}

// do, sets up a worker pool of wp.workers workers using the worker pools config. Each worker listens for jobs to
// process until the worker channel is closed.
//
// Should the context be cancelled whilst listening, it will close the worker channel and stop jobs from being sent
// down it gracefully. Any jobs that were waiting to be processed whilst the context was cancelled, will not be
// attempted. Any jobs that were already being processed will only be shut down if their work method cancels on
// context being cancelled.
func (wp *WorkerPool) do(dr *Dispatcher) {
	var (
		ctx = dr.ctx
	)

	go func() {
		// add a timeout to the worker pool if configured
		if wp.poolTimeout != nil {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, *wp.poolTimeout)
			defer cancel()
		}

		// defer functions are first in last out, so the ctx cancel will only be called after the receiver
		// chan has closed.
		defer func() {
			// close the receiver when all jobs have been processed.
			dr.wait()
			close(dr.rcv)
		}()

		for i := 0; i < wp.workers; i++ {
			dr.listen(ctx)
		}
	}()
}

// Dispatcher allows a user to Dispatch jobs to be processed concurrently and Receive any processed jobs.
type Dispatcher struct {
	ctx context.Context
	// receiver channel to receive processed jobs on.
	rcv chan any
	// worker channel to dispatch jobs down.
	wc chan Worker
	// the timeout to process a job before cancelling it.
	timeout time.Duration
	// whether the worker chan is closed or not.
	closed bool

	// waitGroup for the workers to signal when they have finished processing jobs.
	wg sync.WaitGroup
	// RW Mutex for performance when dispatching. This also lets closing prioritise locking over dispatching.
	mu sync.RWMutex
}

// Dispatch takes a worker and sends it down the channel. it will return an error if the channel is closed.
func (dr *Dispatcher) Dispatch(w Worker) error {
	dr.mu.RLock()
	defer dr.mu.RUnlock()
	if dr.closed {
		return errors.New("dispatcher closed")
	}

	dr.wc <- w

	return nil
}

// Receive implements the receiver interface, it returns a channel to listen for processed jobs on.
func (dr *Dispatcher) Receive() <-chan any {
	return dr.rcv
}

// Done signals all jobs have been dispatched and closes the dispatchers' worker channel.
func (dr *Dispatcher) Done() { dr.close() }

// listen for jobs to process from the worker chan.
func (dr *Dispatcher) listen(ctx context.Context) {
	// register worker.
	dr.add()
	go func() {
		// signal work is finished.
		defer dr.done()

		for {
			select {
			case <-ctx.Done():
				// close the dispatcher early to stop receiving new jobs, as context has been cancelled.
				dr.close()
				return
			case w, open := <-dr.wc:
				if !open {
					// no more jobs, shut down.
					return
				}

				// always add a timeout to any work methods.
				wCtx, wCancel := context.WithTimeout(ctx, dr.timeout)

				// process the required job and send the result down the receiver.
				dr.rcv <- w.Work(wCtx)

				wCancel()
			}
		}
	}()
}

// close the dispatchers' worker channel if it hasn't already been closed.
func (dr *Dispatcher) close() {
	// lock our dispatcher before closing
	dr.mu.Lock()
	defer dr.mu.Unlock()
	if dr.closed {
		return
	}
	dr.closed = true
	close(dr.wc)
}

func (dr *Dispatcher) add()  { dr.wg.Add(1) }
func (dr *Dispatcher) wait() { dr.wg.Wait() }
func (dr *Dispatcher) done() { dr.wg.Done() }
