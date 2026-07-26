package migrations

import "github.com/velocitykode/velocity/orm/migrate"

func init() {
	migrate.Register(&migrate.Migration{
		Version:     "20260726113035",
		Description: "Fix jobs timestamp columns",
		Up: func(m *migrate.Migrator) error {
			// The original create-jobs migration declared scheduled_at /
			// reserved_at / failed_at as varchar. The database queue driver
			// compares scheduled_at against a real timestamp on every worker
			// poll, and postgres has no varchar <= timestamp operator - so on
			// a database built from that shape every poll errors and no job is
			// ever popped, while pushes keep landing rows (attempts stays 0
			// forever). The create migration now declares timestamps, but a
			// database that already recorded its version never re-runs it, so
			// this migration re-shapes those databases.
			//
			// Drop and recreate rather than ALTER: queue rows are transient
			// work, not state - any rows present were never processed (the
			// broken shape is what prevented processing), and a USING cast of
			// varchar RFC3339 strings buys nothing worth the per-driver cast
			// syntax. failed_jobs rides along for the same reason.
			if err := m.DropTable("jobs"); err != nil {
				return err
			}
			if err := m.DropTable("failed_jobs"); err != nil {
				return err
			}

			if err := m.CreateTable("jobs", func(t *migrate.TableBuilder) {
				t.ID()
				t.String("queue", 255)
				t.JSON("payload")
				t.Integer("attempts").Default("0")
				t.Timestamp("scheduled_at")
				t.Timestamp("reserved_at").Nullable()
				t.String("reserved_by", 255).Nullable()
				t.Timestamp("failed_at").Nullable()
				t.String("failed_reason", 5000).Nullable()
				t.Timestamps()
			}); err != nil {
				return err
			}

			// Same pop-path index as the create migration: equality on queue,
			// then both sort columns, so the worker's reserve query walks the
			// index and stops at the first candidate.
			if err := m.Index("jobs", "queue", "scheduled_at", "id"); err != nil {
				return err
			}

			return m.CreateTable("failed_jobs", func(t *migrate.TableBuilder) {
				t.ID()
				t.String("queue", 255)
				t.JSON("payload")
				t.String("exception", 10000)
				t.Timestamps()
			})
		},
		Down: func(m *migrate.Migrator) error {
			// The Up is already the repair; there is no varchar shape worth
			// restoring. Down intentionally leaves the corrected tables.
			return nil
		},
	})
}
