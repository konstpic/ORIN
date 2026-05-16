// Package api is the HTTP gateway: chi router, REST handlers, WebSocket
// gateway, and the OpenAPI 3 spec (served at /api/openapi.yaml).
package api

import (
	_ "embed"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/k8s-ui/k8s-ui/internal/auth"
	"github.com/k8s-ui/k8s-ui/internal/config"
	"github.com/k8s-ui/k8s-ui/internal/controller"
	"github.com/k8s-ui/k8s-ui/internal/crypto"
	"github.com/k8s-ui/k8s-ui/internal/k8s"
	"github.com/k8s-ui/k8s-ui/internal/metrics"
	"github.com/k8s-ui/k8s-ui/internal/notify"
	"github.com/k8s-ui/k8s-ui/internal/reposerver"
	"github.com/k8s-ui/k8s-ui/internal/store"
	"github.com/k8s-ui/k8s-ui/internal/ws"
)

//go:embed openapi.yaml
var openAPISpec []byte

// ServerOptions injects every subsystem the HTTP layer needs.
type ServerOptions struct {
	Config     *config.Config
	Store      *store.Store
	Cipher     *crypto.Cipher
	Cluster    *k8s.ClusterManager
	Repo       *reposerver.Server
	Hub        *ws.Hub
	Controller *controller.Controller
	Notifier   *notify.Dispatcher
	TokenAuth  *auth.TokenAuth
}

// Server bundles options + handlers for the HTTP gateway.
type Server struct {
	opts ServerOptions
}

// NewServer constructs a Server.
func NewServer(opts ServerOptions) *Server { return &Server{opts: opts} }

// Handler returns the configured chi router.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(15 * time.Minute))
	r.Use(corsMiddleware())

	// Public.
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	r.Get("/readyz", s.handleReadyz)
	r.Get("/api/openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write(openAPISpec)
	})
	r.Post("/api/v1/auth/login", s.handleLogin)
	r.Handle("/metrics", promhttp.Handler())

	// Authenticated.
	r.Group(func(r chi.Router) {
		// Use TokenAuth if available, fall back to StaticToken for backward compat
		if s.opts.TokenAuth != nil {
			r.Use(s.opts.TokenAuth.Middleware())
		} else {
			r.Use(auth.StaticToken(s.opts.Config.AdminToken))
		}
		r.Use(prometheusHTTPMiddleware)
		r.Get("/api/v1/auth/userinfo", s.handleUserInfo)

		r.Get("/api/v1/applications", s.listApplications)
		r.Post("/api/v1/applications", s.createApplication)
		r.Post("/api/v1/application-batches", s.createApplicationBatch)
		r.Post("/api/v1/argo-import", s.importArgoApplications)
		r.Get("/api/v1/applications/{name}", s.getApplication)
		r.Put("/api/v1/applications/{name}", s.updateApplication)
		r.Delete("/api/v1/applications/{name}", s.deleteApplication)
		r.Post("/api/v1/applications/{name}/sync", s.syncApplication)
		r.Delete("/api/v1/applications/{name}/sync/{syncId}", s.cancelSync)
		r.Post("/api/v1/applications/{name}/refresh", s.refreshApplication)
		r.Get("/api/v1/applications/{name}/manifests", s.appManifests)
		r.Get("/api/v1/applications/{name}/diff", s.appDiff)
		r.Get("/api/v1/applications/{name}/revisions", s.appRevisions)
		r.Get("/api/v1/applications/{name}/revision-diff", s.appRevisionDiff)
		r.Post("/api/v1/applications/{name}/rollback", s.appRollback)
		r.Get("/api/v1/applications/{name}/pods/{pod}/log", s.getApplicationPodLog)
		r.Get("/api/v1/applications/{name}/pods/{pod}/events", s.getApplicationPodEvents)
		r.Get("/api/v1/applications/{name}/pods/{pod}/shell", s.getApplicationPodShell)
		r.Get("/api/v1/applications/{name}/pods/{pod}/exec", s.appPodExecWS)
		r.Delete("/api/v1/applications/{name}/pods/{pod}", s.deleteApplicationPod)
		r.Get("/api/v1/applications/{name}/pods/{pod}", s.getApplicationPod)
		r.Get("/api/v1/applications/{name}/resource-events", s.getApplicationResourceEvents)

		r.Get("/api/v1/applications/{name}/resource-tree", s.appResourceTree)
		r.Get("/api/v1/applications/{name}/network-map", s.getNetworkMap)
		r.Get("/api/v1/applications/{name}/history", s.appHistory)
		r.Get("/api/v1/applications/{name}/events", s.appEventsWS)
		r.Put("/api/v1/applications/{name}/live-resource", s.applyLiveResource)
		r.Delete("/api/v1/applications/{name}/live-resource", s.deleteLiveResource)
		r.Post("/api/v1/applications/{name}/live-resource/sync", s.syncLiveResource)
		r.Post("/api/v1/applications/{name}/live-resource/restart", s.restartLiveResource)

		// Notification configs per application
		r.Get("/api/v1/applications/{name}/notifications", s.listNotificationConfigs)
		r.Post("/api/v1/applications/{name}/notifications", s.createNotificationConfig)
		r.Put("/api/v1/applications/{name}/notifications/{configId}", s.updateNotificationConfig)
		r.Delete("/api/v1/applications/{name}/notifications/{configId}", s.deleteNotificationConfig)

		// Sync hooks per application
		r.Get("/api/v1/applications/{name}/hooks", s.listSyncHooks)
		r.Post("/api/v1/applications/{name}/hooks", s.createSyncHook)
		r.Put("/api/v1/applications/{name}/hooks/{hookId}", s.updateSyncHook)
		r.Delete("/api/v1/applications/{name}/hooks/{hookId}", s.deleteSyncHook)

		r.Get("/api/v1/repositories", s.listRepositories)
		r.Post("/api/v1/repositories", s.createRepository)
		r.Delete("/api/v1/repositories/{id}", s.deleteRepository)

		r.Get("/api/v1/clusters", s.listClusters)
		r.Post("/api/v1/clusters", s.createCluster)
		r.Get("/api/v1/clusters/health", s.listClusterHealth)
		r.Get("/api/v1/clusters/{id}/nodes", s.listClusterNodes)

		r.Get("/api/v1/projects", s.listProjects)
		r.Post("/api/v1/projects", s.createProject)
		r.Put("/api/v1/projects/{name}", s.updateProject)
		r.Get("/api/v1/audit-log", s.exportAuditLog)

		// Global notifications (not app-specific)
		r.Get("/api/v1/notifications", s.listGlobalNotificationConfigs)
		r.Post("/api/v1/notifications", s.createGlobalNotificationConfig)
		r.Put("/api/v1/notifications/{configId}", s.updateGlobalNotificationConfig)
		r.Delete("/api/v1/notifications/{configId}", s.deleteGlobalNotificationConfig)
		r.Post("/api/v1/notifications/test", s.postNotificationTest)

		// RBAC management
		r.Get("/api/v1/rbac/roles", s.listRoles)
		r.Post("/api/v1/rbac/roles", s.createRole)
		r.Get("/api/v1/rbac/roles/{id}", s.getRole)
		r.Put("/api/v1/rbac/roles/{id}", s.updateRole)
		r.Delete("/api/v1/rbac/roles/{id}", s.deleteRole)

		r.Get("/api/v1/rbac/bindings", s.listRoleBindings)
		r.Post("/api/v1/rbac/bindings", s.createRoleBinding)
		r.Put("/api/v1/rbac/bindings/{id}", s.updateRoleBinding)
		r.Delete("/api/v1/rbac/bindings/{id}", s.deleteRoleBinding)

		r.Get("/api/v1/rbac/permissions", s.listPermissions)

		r.Get("/api/v1/users", s.listUsers)
		r.Post("/api/v1/users", s.createUser)
		r.Get("/api/v1/users/{id}", s.getUser)
		r.Put("/api/v1/users/{id}", s.updateUser)
		r.Delete("/api/v1/users/{id}", s.deleteUser)

		// System configuration
		r.Get("/api/v1/system/config", s.getSystemConfig)
		r.Put("/api/v1/system/config", s.updateSystemConfig)
	})

	// Static frontend assets (optional) - SPA fallback to index.html
	if s.opts.Config.WebAssetsDir != "" {
		fs := http.FileServer(http.Dir(s.opts.Config.WebAssetsDir))
		r.Handle("/*", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			// API routes should 404 if not matched above
			if strings.HasPrefix(req.URL.Path, "/api/") {
				http.NotFound(w, req)
				return
			}
			// Try to serve the file
			path := req.URL.Path

			// Check if file exists
			if info, err := http.Dir(s.opts.Config.WebAssetsDir).Open(path); err == nil {
				stat, err := info.Stat()
				info.Close()
				// If it's a file (not directory) and exists, serve it
				if err == nil && !stat.IsDir() {
					fs.ServeHTTP(w, req)
					return
				}
			}

			// File doesn't exist or is a directory - serve index.html for SPA routing
			req.URL.Path = "/"
			fs.ServeHTTP(w, req)
		}))
	}
	return r
}

func prometheusHTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		pat := r.URL.Path
		if rc := chi.RouteContext(r.Context()); rc != nil {
			if p := rc.RoutePattern(); p != "" {
				pat = p
			}
		}
		metrics.HTTPRequests.WithLabelValues(r.Method, pat).Inc()
	})
}

func corsMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Connection, Upgrade")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func (s *Server) handleReadyz(w http.ResponseWriter, _ *http.Request) {
	// MVP: trust the DB pool is healthy if we got this far.
	_, _ = w.Write([]byte("ok"))
}
