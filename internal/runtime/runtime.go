// Package runtime wires the subsystems together for each subcommand entry
// point. The MVP defaults to "all-in-one" which runs apiserver+controller+
// reposerver in a single process sharing one DB pool and one ClusterManager.
package runtime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/sync/errgroup"

	"github.com/orin/orin/internal/api"
	"github.com/orin/orin/internal/auth"
	"github.com/orin/orin/internal/config"
	"github.com/orin/orin/internal/controller"
	"github.com/orin/orin/internal/crypto"
	"github.com/orin/orin/internal/domain"
	"github.com/orin/orin/internal/grpcserver"
	"github.com/orin/orin/internal/k8s"
	"github.com/orin/orin/internal/leaderelection"
	"github.com/orin/orin/internal/notify"
	"github.com/orin/orin/internal/rbac"
	"github.com/orin/orin/internal/reposerver"
	"github.com/orin/orin/internal/store"
	"github.com/orin/orin/internal/ws"
)

// RunAPIServer starts only the API server.
func RunAPIServer(ctx context.Context, cfg *config.Config) error {
	deps, err := buildDeps(ctx, cfg)
	if err != nil {
		return err
	}
	defer deps.Close()

	if err := ensureBootstrap(ctx, deps, cfg); err != nil {
		return err
	}

	return runHTTPServer(ctx, cfg, deps)
}

// RunController starts only the controller with leader election.
// When multiple controller pods are running, only the leader actively
// reconciles. If the leader crashes, the lock is released and a standby takes over.
func RunController(ctx context.Context, cfg *config.Config) error {
	deps, err := buildDeps(ctx, cfg)
	if err != nil {
		return err
	}
	defer deps.Close()

	leader := leaderelection.New(deps.Store.Pool, "orin-controller")
	slog.Info("controller waiting for leader lock")
	return leader.WaitAndRun(ctx, 10*time.Second, func(ctx context.Context) error {
		slog.Info("controller became leader, starting reconcile loops")
		return deps.Controller.Run(ctx)
	})
}

// RunRepoServer starts the standalone gRPC repo server.
func RunRepoServer(ctx context.Context, cfg *config.Config) error {
	engine, err := reposerver.NewEngine(cfg.RepoCacheDir)
	if err != nil {
		return fmt.Errorf("create engine: %w", err)
	}
	svc := grpcserver.New(engine)
	addr := envOr("REPO_SERVER_ADDR", ":50051")
	slog.Info("starting repo server", "addr", addr)
	errCh := make(chan error, 1)
	go func() { errCh <- svc.ListenAndServe(addr) }()
	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// RunAllInOne is the default MVP entry point.
func RunAllInOne(ctx context.Context, cfg *config.Config) error {
	deps, err := buildDeps(ctx, cfg)
	if err != nil {
		return err
	}
	defer deps.Close()

	if err := ensureBootstrap(ctx, deps, cfg); err != nil {
		return err
	}

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return deps.Controller.Run(gctx) })
	g.Go(func() error { return runHTTPServer(gctx, cfg, deps) })
	return g.Wait()
}

// MigrateUp runs all up-migrations.
func MigrateUp(_ context.Context, cfg *config.Config) error {
	return store.Migrate(cfg.DatabaseURL)
}

// MigrateDown rolls back one migration.
func MigrateDown(_ context.Context, cfg *config.Config) error {
	return store.MigrateDown(cfg.DatabaseURL)
}

// MigrateStatus reports the current migration state.
func MigrateStatus(_ context.Context, cfg *config.Config) (uint, bool, error) {
	return store.MigrateStatus(cfg.DatabaseURL)
}

// deps groups the shared subsystems used by both the HTTP server and the
// controller.
type deps struct {
	Store      *store.Store
	Cipher     *crypto.Cipher
	Cluster    *k8s.ClusterManager
	Repo       *reposerver.Server
	Hub        *ws.Hub
	Controller *controller.Controller
	Notifier   *notify.Dispatcher
}

func (d *deps) Close() {
	if d.Repo != nil {
		_ = d.Repo.Close()
	}
	if d.Cluster != nil {
		d.Cluster.Close()
	}
	if d.Store != nil {
		d.Store.Close()
	}
}

func buildDeps(ctx context.Context, cfg *config.Config) (*deps, error) {
	cipher, err := crypto.New(cfg.EncryptionKey)
	if err != nil {
		return nil, fmt.Errorf("init crypto: %w", err)
	}

	st, err := store.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect db: %w", err)
	}

	if err := store.Migrate(cfg.DatabaseURL); err != nil {
		return nil, fmt.Errorf("apply migrations: %w", err)
	}

	cm, err := k8s.NewClusterManager(cfg)
	if err != nil {
		st.Close()
		return nil, fmt.Errorf("init cluster manager: %w", err)
	}

	rs, err := reposerver.New(ctx, cfg, st, cipher)
	if err != nil {
		cm.Close()
		st.Close()
		return nil, fmt.Errorf("init repo server: %w", err)
	}

	hub := ws.NewHub()
	notifier := notify.New()
	ctrl := controller.New(cfg, st, cm, rs, hub, cipher, notifier)

	return &deps{
		Store:      st,
		Cipher:     cipher,
		Cluster:    cm,
		Repo:       rs,
		Hub:        hub,
		Controller: ctrl,
		Notifier:   notifier,
	}, nil
}

func runHTTPServer(ctx context.Context, cfg *config.Config, d *deps) error {
	tokenAuth := auth.NewTokenAuth(d.Store.Pool, cfg.AdminToken, 5*time.Minute)
	handler := api.NewServer(api.ServerOptions{
		Config:     cfg,
		Store:      d.Store,
		Cipher:     d.Cipher,
		Cluster:    d.Cluster,
		Repo:       d.Repo,
		Hub:        d.Hub,
		Controller: d.Controller,
		Notifier:   d.Notifier,
		TokenAuth:  tokenAuth,
	}).Handler()

	srv := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: handler,
	}
	errCh := make(chan error, 1)
	go func() {
		slog.Info("api server listening", "addr", cfg.HTTPAddr)
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// ensureBootstrap creates the in-cluster Cluster row on first launch and seeds
// default RBAC roles.
func ensureBootstrap(ctx context.Context, d *deps, cfg *config.Config) error {
	if _, err := d.Store.Clusters.GetByName(ctx, "in-cluster"); err == nil {
		return nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return err
	}
	cl := &domain.Cluster{
		Name:      "in-cluster",
		ServerURL: d.Cluster.ServerURL(),
		InCluster: true,
	}
	slog.Info("bootstrapping in-cluster row", "server", cl.ServerURL)
	_ = cfg
	if err := d.Store.Clusters.Upsert(ctx, cl); err != nil {
		return err
	}

	// Seed default RBAC roles
	return seedRBACRoles(ctx, d, cfg)
}

// seedRBACRoles creates the built-in admin, editor, and viewer roles if they
// don't already exist, and binds the first admin user to the admin role.
func seedRBACRoles(ctx context.Context, d *deps, cfg *config.Config) error {
	for _, preset := range rbac.DefaultRolePresets() {
		_, err := d.Store.Roles.GetByName(ctx, preset.Name)
		if err == nil {
			continue // already exists
		}
		if !errors.Is(err, store.ErrNotFound) {
			return err
		}

		role := &rbac.Role{
			ID:          "rbac-" + preset.Name,
			Name:        preset.Name,
			DisplayName: preset.DisplayName,
			Description: preset.Description,
			Permissions: preset.Permissions,
			BuiltIn:     preset.BuiltIn,
		}
		if err := d.Store.Roles.Create(ctx, role); err != nil {
			slog.Warn("failed to seed role", "role", preset.Name, "error", err)
		}
	}

	// Ensure the admin user exists and is bound to the admin role
	var userID string
	var existingHash string
	err := d.Store.Pool.QueryRow(ctx, `
		SELECT id, COALESCE(token_hash, '') FROM users WHERE role = 'admin' AND active = true LIMIT 1
	`).Scan(&userID, &existingHash)
	if err != nil {
		// Create a bootstrap admin user with the static admin token
		userID = "user-admin-bootstrap"
		tokenHash := hashTokenForSeed(cfg.AdminToken)
		_, _ = d.Store.Pool.Exec(ctx, `
			INSERT INTO users (id, email, display_name, role, token_hash, active)
			VALUES ($1, 'admin@orin.local', 'Administrator', 'admin', $2, true)
			ON CONFLICT (email) DO NOTHING
		`, userID, tokenHash)
	} else if existingHash == "" && cfg.AdminToken != "" {
		// Admin user exists but has no token_hash — set it
		tokenHash := hashTokenForSeed(cfg.AdminToken)
		_, _ = d.Store.Pool.Exec(ctx, `UPDATE users SET token_hash = $1 WHERE id = $2`, tokenHash, userID)
	}

	// Check if admin already has a role binding
	var bindingCount int
	err = d.Store.Pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM role_bindings WHERE user_id = $1
	`, userID).Scan(&bindingCount)
	if err == nil && bindingCount == 0 {
		adminRole, err := d.Store.Roles.GetByName(ctx, "admin")
		if err == nil {
			binding := &rbac.RoleBinding{
				ID:       "binding-admin",
				UserID:   userID,
				RoleID:   adminRole.ID,
				Projects: []string{"*"},
			}
			_ = d.Store.RoleBindings.Create(ctx, binding)
		}
	}

	return nil
}

func hashTokenForSeed(token string) string {
	if token == "" {
		return ""
	}
	h, err := bcrypt.GenerateFromPassword([]byte(token), bcrypt.DefaultCost)
	if err != nil {
		return ""
	}
	return string(h)
}
