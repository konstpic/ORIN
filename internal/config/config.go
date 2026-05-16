// Package config centralises environment-driven configuration for all
// k8s-ui subcommands. Values are read once at process startup and validated
// before any subsystem starts.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Version is overridden at build time via -ldflags.
var Version = "dev"

// Config holds settings shared across apiserver/controller/reposerver.
type Config struct {
	// HTTP
	HTTPAddr string

	// Persistence
	DatabaseURL string

	// Authentication. AdminToken is a single static bearer token used for the
	// MVP. When OIDCIssuerURL and OIDCClientID are set, an OIDC login flow can
	// be wired without dropping static-token bootstrap (see docs).
	AdminToken string
	OIDCIssuerURL string
	OIDCClientID  string
	OIDCAudience  string

	// EncryptionKey is a 32-byte hex string used to AES-GCM encrypt secret
	// columns (repo credentials, kubeconfigs).
	EncryptionKey string

	// Kubernetes
	KubeconfigPath string // empty = in-cluster
	InCluster      bool   // explicit override

	// Repo server
	RepoCacheDir       string
	RepoPollInterval   time.Duration
	RepoRenderTimeout  time.Duration

	// Controller
	ReconcileWorkers   int
	ReconcileResync    time.Duration
	// SyncApplyRetries is per-resource apply attempts on transient errors (>=1).
	SyncApplyRetries int
	// AutoSyncGracePeriod is the duration after a manual live-apply during which
	// auto-sync (self-heal) is suppressed, allowing the user's edit to persist
	// and show as OutOfSync instead of being immediately reverted.
	AutoSyncGracePeriod time.Duration
	// SyncDenyRangeUTC blocks manual and auto sync during a daily UTC window,
	// format "22:00-06:00". Empty disables.
	SyncDenyRangeUTC string
	syncDeny         *syncDenyRange

	// Frontend assets directory (optional embedded UI fallback)
	WebAssetsDir string

	// AppsCatalog: optional Git-driven registry of Application rows (declarative
	// "app of apps"). When AppsCatalogRepoURL is set, the controller periodically
	// reads AppsCatalogPath from that registered repository and creates/updates
	// matching applications. Entries removed from the file are not deleted.
	AppsCatalogRepoURL  string
	AppsCatalogPath     string
	AppsCatalogRevision string
	AppsCatalogInterval time.Duration
}

// Load returns a Config populated from environment variables with sensible
// MVP defaults.
func Load() (*Config, error) {
	cfg := &Config{
		HTTPAddr:          envOr("HTTP_ADDR", ":8080"),
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		AdminToken:        os.Getenv("ADMIN_TOKEN"),
		EncryptionKey:     os.Getenv("ENCRYPTION_KEY"),
		KubeconfigPath:    os.Getenv("KUBECONFIG"),
		InCluster:         envBool("IN_CLUSTER", false),
		RepoCacheDir:      envOr("REPO_CACHE_DIR", "/var/cache/k8s-ui/repos"),
		RepoPollInterval:  envDuration("REPO_POLL_INTERVAL", 3*time.Minute),
		RepoRenderTimeout: envDuration("REPO_RENDER_TIMEOUT", 60*time.Second),
		ReconcileWorkers:  envInt("RECONCILE_WORKERS", 10),
		ReconcileResync:   envDuration("RECONCILE_RESYNC", 3*time.Minute),
		SyncApplyRetries:  envInt("SYNC_APPLY_RETRIES", 1),
		AutoSyncGracePeriod: envDuration("AUTO_SYNC_GRACE_PERIOD", 30*time.Minute),
		SyncDenyRangeUTC:  os.Getenv("SYNC_DENY_RANGE_UTC"),
		OIDCIssuerURL:     os.Getenv("OIDC_ISSUER_URL"),
		OIDCClientID:      os.Getenv("OIDC_CLIENT_ID"),
		OIDCAudience:      os.Getenv("OIDC_AUDIENCE"),
		WebAssetsDir:      os.Getenv("WEB_ASSETS_DIR"),
		AppsCatalogRepoURL:  os.Getenv("APPS_CATALOG_REPO_URL"),
		AppsCatalogPath:     envOr("APPS_CATALOG_PATH", "k8s-ui/apps.yaml"),
		AppsCatalogRevision: envOr("APPS_CATALOG_REVISION", "HEAD"),
		AppsCatalogInterval: envDuration("APPS_CATALOG_INTERVAL", 5*time.Minute),
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	deny, err := parseSyncDenyRangeUTC(cfg.SyncDenyRangeUTC)
	if err != nil {
		return nil, err
	}
	cfg.syncDeny = deny
	return cfg, nil
}

// SyncDeniedAt reports whether sync should be refused at time t (UTC window).
func (c *Config) SyncDeniedAt(t time.Time) bool {
	if c == nil || c.syncDeny == nil {
		return false
	}
	return c.syncDeny.blocksUTC(t)
}

func (c *Config) validate() error {
	var errs []error
	if c.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}
	if c.AdminToken == "" {
		errs = append(errs, errors.New("ADMIN_TOKEN is required for MVP auth"))
	}
	if c.EncryptionKey == "" {
		errs = append(errs, errors.New("ENCRYPTION_KEY is required (32-byte hex)"))
	} else if len(c.EncryptionKey) != 64 {
		errs = append(errs, fmt.Errorf("ENCRYPTION_KEY must be 32 bytes hex-encoded (got %d chars)", len(c.EncryptionKey)))
	}
	if c.ReconcileWorkers < 1 {
		errs = append(errs, errors.New("RECONCILE_WORKERS must be >= 1"))
	}
	if c.SyncApplyRetries < 1 {
		c.SyncApplyRetries = 1
	}
	if c.AppsCatalogRepoURL != "" {
		if c.AppsCatalogInterval < 10*time.Second {
			errs = append(errs, errors.New("APPS_CATALOG_INTERVAL must be >= 10s when APPS_CATALOG_REPO_URL is set"))
		}
	}
	return errors.Join(errs...)
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envBool(k string, def bool) bool {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func envDuration(k string, def time.Duration) time.Duration {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}
