package app

import (
	"context"
	"encoding/json"
	"html/template"
	"net/http"
	"os"
	"strings"
	"time"

	"velship-velocity-react/config"
	"velship-velocity-react/internal/jobs"
	"velship-velocity-react/internal/models"
	"velship-velocity-react/internal/sessionstore"

	"github.com/velocitykode/velocity"
	velocityapp "github.com/velocitykode/velocity/app"
	"github.com/velocitykode/velocity/auth"
	"github.com/velocitykode/velocity/bond/vite"
	"github.com/velocitykode/velocity/cache"
	"github.com/velocitykode/velocity/csrf"
	"github.com/velocitykode/velocity/queue"
	"github.com/velocitykode/velocity/view"
)

// Configure registers the app's modules. main.go passes this to
// v.Modules(...) - the framework calls Init on every module during
// bootstrap, then Start once Init has finished for all of them.
func Configure(reg *velocity.ModuleRegistry) {
	reg.Add(&AppModule{})
}

// AppModule wires the view engine for this application. CSRF and auth
// schemes are built by the framework from env vars during velocity.New(),
// so we only need to stand up the Inertia view engine and share the
// CSRF token into its props.
type AppModule struct {
	// worker is the in-process queue worker, stopped on Shutdown.
	worker *queue.Worker
}

// Init runs before any module's Start. The framework already built
// CSRF, Auth, and the session scheme, so the work here is pointing auth
// at this app's own model and filling the queue job registry.
func (p *AppModule) Init(s *velocity.Services) error {
	// The model is a type parameter, not configuration: the ORM resolves
	// its table from a compile-time type, so a wrong model is a compile
	// error. Installing the module re-points every scheme velocity.New
	// already built.
	if err := velocity.SetAuthModel[models.User](s); err != nil {
		return err
	}
	registerJobs()
	return nil
}

// registerJobs teaches the queue how to rebuild each job type from its
// persisted payload.
//
// A durable driver (redis, database) carries only JSON across the process
// boundary, so the worker reconstructs the job through this registry rather
// than receiving the original pointer. Without an entry a pop fails with
// ErrJobNotFound and the job is routed to failed_jobs - it does not run and
// does not silently no-op.
//
// RegisterJob derives the registry key from the type itself, so the push side
// and the decode side cannot drift the way a hand-written string key can.
func registerJobs() {
	queue.RegisterJob(func(data []byte) (*jobs.SendWelcomeEmail, error) {
		j := &jobs.SendWelcomeEmail{}
		return j, json.Unmarshal(data, j)
	})
}

// Start wires the view engine and the server-side session store - runs
// after every module's Init.
func (p *AppModule) Start(s *velocity.Services) error {
	if err := bootstrapSessionStore(s); err != nil {
		return err
	}
	if err := bootstrapView(s); err != nil {
		return err
	}
	worker, err := bootstrapQueueWorker(s)
	if err != nil {
		return err
	}
	p.worker = worker
	return nil
}

// bootstrapQueueWorker runs a queue worker inside the web process.
//
// The default queue driver holds jobs in memory, so a worker started as a
// separate `vel queue work` process would poll its own empty queue and never
// see anything this process pushed. Co-locating the worker is what makes a
// dispatch actually run - and what lets the job lifecycle (job.processing,
// job.processed, job.failed) reach the event dispatcher, since only a running
// worker emits those three.
//
// A single serial worker: this exists to make queue behaviour observable, not
// to carry throughput. Returns nil when no queue is configured.
func bootstrapQueueWorker(s *velocity.Services) (*queue.Worker, error) {
	if s.Queue == nil {
		return nil, nil
	}
	worker := queue.NewWorker(
		s.Queue,
		"default",
		func(job queue.Job) error { return job.Handle() },
		queue.WithWorkerLogger(s.Log),
	)

	// Registering the worker is what gets its event dispatcher wired. The
	// framework re-runs its dispatcher sweep over registered components after
	// every module has booted, and queue.Worker implements
	// contract.EventDispatcherAware - so job.processing, job.processed and
	// job.failed reach listeners. A hand-constructed worker that is never
	// registered still runs jobs, silently: its dispatch is nil-safe, so the
	// work happens and only the observability disappears.
	if err := velocityapp.Register(s, worker); err != nil {
		return nil, err
	}

	// The worker owns teardown through Stop, which cancels this context and
	// waits for in-flight jobs; the loop is not tied to a request lifetime.
	worker.Start(context.Background())
	return worker, nil
}

// bootstrapSessionStore installs the cache-backed ServerSessionStore on the
// auth.Manager. Production is secure-by-default: cookie-only sessions cannot
// propagate revocation across processes, so velocity's boot-time H-04 guard
// refuses a production boot without a server store. With CACHE_DRIVER=redis
// the records survive restarts and stay coherent across instances. Skipped
// silently when auth or cache is not wired (JWT-only or test bootstraps).
func bootstrapSessionStore(s *velocity.Services) error {
	authManager, ok := s.Auth.(*auth.Manager)
	if !ok || authManager == nil {
		return nil
	}
	cm, ok := s.Cache.(cache.CacheManager)
	if !ok || cm == nil {
		return nil
	}
	store, err := sessionstore.New(cm)
	if err != nil {
		return err
	}
	// The manager propagates the store to every scheme implementing
	// auth.ServerSessionStoreReceiver; the session scheme consults it on
	// every authenticated request and on Login/Logout.
	authManager.SetServerSessionStore(store)
	return nil
}

func (p *AppModule) Shutdown(_ context.Context) error {
	if p.worker != nil {
		// Stop cancels the worker context and waits for in-flight jobs, so
		// shutdown does not truncate a job mid-flight.
		p.worker.Stop()
	}
	return nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envDurationOrDefault(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func bootstrapView(s *velocity.Services) error {
	// view.Config.RootTemplate takes the HTML content string, not a path.
	// Read the file ourselves so bond can parse + validate it.
	templateBytes, err := os.ReadFile(config.GetViewTemplate())
	if err != nil {
		return err
	}

	// Vite helper exposes {{ vite "resources/js/app.tsx" }} to the
	// root template. In dev (public/hot exists) it emits dev-server
	// tags; in prod it reads public/build/.vite/manifest.json and
	// emits hashed asset URLs with modulepreload + stylesheet links.
	viteHelper := vite.New()

	viewConfig := view.Config{
		RootTemplate: string(templateBytes),
		Version:      config.GetViewVersion(),
		SSREnabled:   os.Getenv("VIEW_SSR_ENABLED") == "true",
		SSRURL:       envOrDefault("VIEW_SSR_URL", "http://127.0.0.1:13714"),
		SSRTimeout:   envDurationOrDefault("VIEW_SSR_TIMEOUT", 3*time.Second),
		Funcs: template.FuncMap{
			"vite": func(entrypoints ...string) template.HTML {
				out, _ := viteHelper.Tags(entrypoints...)
				return out
			},
			// React Fast Refresh preamble - emits a script in dev
			// mode, empty in prod. Must precede {{ vite ... }} so the
			// preamble runs before @vite/client.
			"viteReactRefresh": func() template.HTML {
				out, _ := viteHelper.ReactRefreshTag()
				return out
			},
		},
	}
	if except := os.Getenv("VIEW_SSR_EXCEPT"); except != "" {
		for _, p := range strings.Split(except, ",") {
			if p = strings.TrimSpace(p); p != "" {
				viewConfig.SSRExcept = append(viewConfig.SSRExcept, p)
			}
		}
	}

	engine, err := view.NewEngine(viewConfig)
	if err != nil {
		return err
	}

	s.View = engine

	engine.SetSharePropsFunc(func(r *http.Request) (view.Props, error) {
		props := view.Props{}
		if token, err := csrf.TokenForRequest(r); err == nil && token != "" {
			props["csrf_token"] = token
		}
		return props, nil
	})

	return nil
}
