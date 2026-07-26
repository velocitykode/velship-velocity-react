package jobs

import (
	"context"
	"errors"
	"time"
)

// SendWelcomeEmail stands in for post-registration work. It sends nothing:
// the point is to exercise the queue lifecycle so a dispatch surfaces as
// job.queued -> job.processing -> job.processed, with a duration large
// enough to be distinguishable from instantaneous work.
type SendWelcomeEmail struct {
	Email string
	// Fail makes the job return an error so the failure lane (retries, then
	// job.failed) can be exercised without a second job type.
	Fail bool
}

// HandleCtx executes the job with the worker's context.
//
// Implementing queue.HandleCtxer rather than only Handle buys two things: the
// context carries the dispatching request's trace ids, so the job's work joins
// that trace instead of starting an orphan one, and the work aborts on worker
// shutdown or job timeout instead of running to completion regardless.
func (j *SendWelcomeEmail) HandleCtx(ctx context.Context) error {
	// Stand-in for the delivery call: a fixed, visible cost so the job's
	// span has a duration worth looking at instead of rounding to zero.
	// Selecting on ctx.Done is what makes it cancellable.
	select {
	case <-time.After(250 * time.Millisecond):
	case <-ctx.Done():
		return ctx.Err()
	}

	if j.Fail {
		return errors.New("welcome email delivery rejected by upstream")
	}
	return nil
}

// Handle satisfies queue.Job for callers that invoke a job directly rather
// than through a worker. The worker prefers HandleCtx.
func (j *SendWelcomeEmail) Handle() error {
	return j.HandleCtx(context.Background())
}

// Failed is called when the job has exceeded its max attempts.
func (j *SendWelcomeEmail) Failed(err error) {
	// Terminal handler. Deliberately empty: the queue emits job.failed for
	// this job, which is the signal this type exists to produce.
}

// MaxAttempts returns the maximum number of times this job may be attempted.
func (j *SendWelcomeEmail) MaxAttempts() int {
	return 3
}
