package pool

import "context"

// Worker interface allows a user to perform jobs.
//
// The user is expected to handle any errors that occur during the Work method.
type Worker interface {
	Work(context.Context) any
}
