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
//  nil check in methods for fields, not struct

var (
	// ErrDispatcherClosed signals the dispatcher is closed and any further jobs will not be dispatched and
	// therefore processed.
	ErrDispatcherClosed = errors.New("dispatcher closed")
)

// Dispatcher allows a user to Dispatch jobs to be processed concurrently and Receive any processed jobs.
type Dispatcher struct {
	ctx context.Context
	// receiver channel to receive processed jobs on.
	rcv chan any
	// worker channel to dispatch jobs down.
	wc chan Worker
	// error stores the latest error received from the dispatcher.
	err error
	// the timeout to process a job before cancelling it.
	timeout time.Duration
	// whether the worker chan is closed or not.
	closed bool

	// waitGroup for the workers to signal when they have finished processing jobs.
	wg sync.WaitGroup
	// RW Mutex for performance when dispatching. This also lets closing prioritise locking over dispatching.
	mu sync.RWMutex
}

// Dispatch takes a worker and sends it down the channel. it will return an error if the dispatcher is closed.
func (d *Dispatcher) Dispatch(w Worker) error {
	if err := d.isOpen(); err != nil {
		return err
	}

	d.wc <- w

	return nil
}

// checks if the dispatcher is still open for dispatching jobs, if not, the error is returned.
func (d *Dispatcher) isOpen() error {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.closed {
		return ErrDispatcherClosed
	}

	return d.err
}

// Receive returns a channel to listen for processed jobs on.
//
// The user can safely listen on this channel until it is closed, either by the user calling Done() or internally
// due to an error.
//
// The user should make a call to Err() once they have finished receiving to check if any errors were encountered
// during the dispatching or processing of the jobs.
func (d *Dispatcher) Receive() <-chan any {
	return d.rcv
}

// Done signals all jobs have been dispatched and closes the dispatchers' worker channel.
func (d *Dispatcher) Done() { d.close() }

// Err returns the last error received from the dispatcher.
func (d *Dispatcher) Err() error {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.err
}

// do sets up a worker pool of wp.workers workers using the worker pools config. Each worker listens for jobs to
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
		if wp.poolTimeout() != nil {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, *wp.poolTimeout())
			defer cancel()
		}

		// defer functions are first in last out, so the ctx cancel will only be called after the receiver
		// chan has closed.
		defer func() {
			// close the receiver when all workers have exited.
			dr.wg.Wait()
			close(dr.rcv)
		}()

		for i := 0; i < wp.workers(); i++ {
			dr.listen(ctx)
		}
	}()
}

// listen for jobs to process from the worker chan.
func (d *Dispatcher) listen(ctx context.Context) {
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()

		for {
			select {
			case <-ctx.Done():
				d.setErr(ctx.Err())
				return
			case w, open := <-d.wc:
				if !open {
					// no more jobs, shut down.
					return
				}

				d.process(ctx, w)
			}
		}
	}()
}

// process the required job and send the result down the receiver.
func (d *Dispatcher) process(ctx context.Context, w Worker) {
	// always add a timeout to any jobs.
	wCtx, wCancel := context.WithTimeout(ctx, d.timeout)
	defer wCancel()

	d.rcv <- w.Work(wCtx)
}

func (d *Dispatcher) setErr(err error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.err = err
}

// close the dispatchers' worker channel if it hasn't already been closed.
func (d *Dispatcher) close() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.closed {
		return
	}
	d.closed = true
	close(d.wc)
}
