package handlers

import (
	"velship-velocity-react/internal/jobs"

	"github.com/velocitykode/velocity/router"
)

// DispatchWelcomeEmail pushes a SendWelcomeEmail onto the queue and returns
// immediately, so the request span closes while the job is still running on
// the worker. That separation is the point: the request and the job appear as
// distinct units of work rather than one blocking handler.
//
// ?fail=1 dispatches a job that always errors, driving the retry path through
// to job.failed.
func DispatchWelcomeEmail(ctx *router.Context) error {
	job := &jobs.SendWelcomeEmail{
		Email: "probe@example.com",
		Fail:  ctx.Request.URL.Query().Get("fail") == "1",
	}

	if err := ctx.Services().Queue.PushCtx(ctx.Request.Context(), job); err != nil {
		ctx.Log().Error("failed to dispatch welcome email job", "error", err)
		return ctx.JSON(500, map[string]any{"error": "dispatch failed"})
	}

	return ctx.JSON(202, map[string]any{
		"dispatched": "SendWelcomeEmail",
		"fail":       job.Fail,
	})
}
