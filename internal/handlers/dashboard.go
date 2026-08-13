package handlers

import (
	"math/rand"
	"time"

	"velship-velocity-react/internal/models"

	"github.com/dustin/go-humanize"
	"github.com/oklog/ulid/v2"
	"github.com/shopspring/decimal"
	"github.com/velocitykode/velocity/auth"
	"github.com/velocitykode/velocity/router"
	"github.com/velocitykode/velocity/view"
)

// bootedAt anchors the uptime stat the dashboard surfaces.
var bootedAt = time.Now()

// Dashboard displays the dashboard
func Dashboard(ctx *router.Context) error {
	user := auth.FromContext(ctx).User(ctx.Request)

	// The ORM-backed user store hands back the app's own model - the
	// framework's AuthUser wrapper is only what a store with no model of
	// its own returns.
	userMap := make(map[string]interface{})
	if u, ok := user.(*models.User); ok && u != nil {
		userMap["id"] = u.ID
		userMap["name"] = u.Name
		userMap["email"] = u.Email
	}

	// Sample stats block: exact decimal arithmetic for a monetary figure and a
	// human-readable uptime, rendered by the dashboard's stats strip.
	mrr := decimal.NewFromInt(1299).Div(decimal.NewFromInt(100)).Mul(decimal.NewFromInt(42))
	stats := map[string]interface{}{
		"mrr":    mrr.StringFixed(2),
		"uptime": humanize.Time(bootedAt),
		// Per-render trace id: sortable, stamps when the dashboard payload was
		// composed so the client can show data freshness.
		"traceId": ulid.MustNew(ulid.Timestamp(time.Now()), rand.New(rand.NewSource(time.Now().UnixNano()))).String(),
	}

	view.Render(ctx, "Dashboard", view.Props{
		"auth": map[string]interface{}{
			"user": userMap,
		},
		"stats": stats,
	})
	return nil
}
