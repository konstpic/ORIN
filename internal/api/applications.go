package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/k8s-ui/k8s-ui/internal/auth"
	"github.com/k8s-ui/k8s-ui/internal/domain"
	"github.com/k8s-ui/k8s-ui/internal/k8s"
	"github.com/k8s-ui/k8s-ui/internal/manifest"
	"github.com/k8s-ui/k8s-ui/internal/project"
	"github.com/k8s-ui/k8s-ui/internal/store"
	apiv1 "github.com/k8s-ui/k8s-ui/pkg/api/v1"
)

func (s *Server) listApplications(w http.ResponseWriter, r *http.Request) {
	apps, err := s.opts.Store.Applications.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	out := make([]apiv1.Application, 0, len(apps))
	for _, a := range apps {
		if u, ok := auth.FromContext(r.Context()); ok && !auth.CanAccessProject(u, a.Project) {
			continue
		}
		dto, err := s.toAPIApp(r.Context(), a)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "convert_failed", err.Error())
			return
		}
		out = append(out, dto)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getApplication(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	app, err := s.opts.Store.Applications.GetByName(r.Context(), name)
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	if !requireProjectAccess(w, r, app.Project) {
		return
	}
	dto, err := s.toAPIApp(r.Context(), app)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "convert_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, dto)
}

func (s *Server) createApplication(w http.ResponseWriter, r *http.Request) {
	var req apiv1.CreateApplicationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	proj := req.Project
	if proj == "" {
		proj = "default"
	}
	if !requireProjectAccess(w, r, proj) {
		return
	}
	app, err := s.persistApplication(r.Context(), req)
	if err != nil {
		if he, ok := err.(*httpCreateErr); ok {
			writeError(w, he.code, he.key, he.msg)
			return
		}
		writeError(w, http.StatusInternalServerError, "create_failed", err.Error())
		return
	}
	s.opts.Controller.EnqueueStatus(app.Name)
	dto, _ := s.toAPIApp(r.Context(), app)
	writeJSON(w, http.StatusCreated, dto)
}

type httpCreateErr struct {
	code int
	key  string
	msg  string
}

func (e *httpCreateErr) Error() string { return e.msg }

func applyHelmValuesJSON(app *domain.Application, raw json.RawMessage) error {
	if raw == nil {
		return nil
	}
	s := bytes.TrimSpace(raw)
	if len(s) == 0 || string(s) == "null" {
		app.HelmValuesJSON = nil
		return nil
	}
	if !json.Valid(s) {
		return &httpCreateErr{http.StatusBadRequest, "invalid_helm_values", "source.helmValues must be valid JSON"}
	}
	app.HelmValuesJSON = append([]byte(nil), s...)
	return nil
}

func (s *Server) persistApplication(ctx context.Context, req apiv1.CreateApplicationRequest) (*domain.Application, error) {
	if req.Name == "" || req.Source.RepoURL == "" || req.Destination.Cluster == "" || req.Destination.Namespace == "" {
		return nil, &httpCreateErr{http.StatusBadRequest, "missing_fields", "name, source.repoUrl, destination.cluster, destination.namespace are required"}
	}
	repo, err := s.opts.Store.Repositories.GetByURL(ctx, req.Source.RepoURL)
	if err != nil {
		return nil, &httpCreateErr{http.StatusBadRequest, "unknown_repo", "register the repository before creating an application"}
	}
	cl, err := s.opts.Store.Clusters.GetByName(ctx, req.Destination.Cluster)
	if err != nil {
		return nil, &httpCreateErr{http.StatusBadRequest, "unknown_cluster", "destination cluster is not registered"}
	}
	if req.Source.TargetRevision == "" {
		req.Source.TargetRevision = "HEAD"
	}
	projectName := req.Project
	if projectName == "" {
		projectName = "default"
	}
	if err := s.enforceProjectSourceAndDest(ctx, projectName, req.Source.RepoURL, cl, req.Destination.Namespace); err != nil {
		return nil, &httpCreateErr{http.StatusForbidden, "project_policy", err.Error()}
	}
	app := &domain.Application{
		Name:           req.Name,
		Project:        projectName,
		RepoID:         repo.ID,
		Path:           req.Source.Path,
		TargetRevision: req.Source.TargetRevision,
		HelmValueFiles: append([]string(nil), req.Source.HelmValueFiles...),
		DestClusterID:  cl.ID,
		DestNamespace:  req.Destination.Namespace,
	}
	if req.SyncPolicy.Automated != nil {
		app.SyncPolicy.Automated = &domain.AutomatedSync{
			Prune:    req.SyncPolicy.Automated.Prune,
			SelfHeal: req.SyncPolicy.Automated.SelfHeal,
		}
	}
	app.SyncPolicy.SyncOptions = append([]string(nil), req.SyncPolicy.SyncOptions...)
	if m := req.SyncPolicy.ManagedNamespaceMetadata; m != nil {
		app.SyncPolicy.ManagedNamespaceMetadata = &domain.ManagedNamespaceMetadata{
			Labels:      maps.Clone(m.Labels),
			Annotations: maps.Clone(m.Annotations),
		}
	}
	app.SyncPolicy.CreateNamespace = req.SyncPolicy.CreateNamespace
	app.SyncPolicy.IgnoreDifferences = apiIgnoreDiffToDomain(req.SyncPolicy.IgnoreDifferences)
	if err := applyHelmValuesJSON(app, req.Source.HelmValues); err != nil {
		return nil, err
	}
	if err := s.opts.Store.Applications.Create(ctx, app); err != nil {
		return nil, err
	}
	return app, nil
}

// enforceProjectSourceAndDest checks source repo URL and destination cluster+namespace
// against the project policy. cl may be nil when only the source needs checking.
func (s *Server) enforceProjectSourceAndDest(ctx context.Context, projectName, repoURL string, cl *domain.Cluster, namespace string) error {
	pr, err := s.opts.Store.Projects.GetByName(ctx, projectName)
	if err != nil {
		if isNotFound(err) {
			return nil // unknown project → no constraints (default-project fallback)
		}
		return err
	}
	enf := project.NewEnforcer(*pr)
	if repoURL != "" {
		if err := enf.CheckSource(repoURL); err != nil {
			return err
		}
	}
	if cl != nil && namespace != "" {
		if err := enf.CheckDestination(cl.Name, cl.ServerURL, namespace); err != nil {
			return err
		}
	}
	return nil
}

// enforceProjectManifests checks rendered manifests against project resource rules.
func (s *Server) enforceProjectManifests(ctx context.Context, projectName string, objs []*unstructured.Unstructured) error {
	if len(objs) == 0 {
		return nil
	}
	pr, err := s.opts.Store.Projects.GetByName(ctx, projectName)
	if err != nil {
		if isNotFound(err) {
			return nil
		}
		return err
	}
	enf := project.NewEnforcer(*pr)
	for _, pErr := range enf.CheckManifests(objs) {
		return pErr
	}
	return nil
}

func isNotFound(err error) bool {
	return errors.Is(err, store.ErrNotFound)
}

func apiIgnoreDiffToDomain(in []apiv1.IgnoreDifferenceRule) []domain.IgnoreDifferenceRule {
	if len(in) == 0 {
		return nil
	}
	out := make([]domain.IgnoreDifferenceRule, len(in))
	for i, r := range in {
		out[i] = domain.IgnoreDifferenceRule{
			Group:        r.Group,
			Kind:         r.Kind,
			Name:         r.Name,
			Namespace:    r.Namespace,
			JSONPointers: append([]string(nil), r.JSONPointers...),
		}
	}
	return out
}

func (s *Server) updateApplication(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	app, err := s.opts.Store.Applications.GetByName(r.Context(), name)
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	if !requireProjectAccess(w, r, app.Project) {
		return
	}
	var req apiv1.UpdateApplicationRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if req.Source.RepoURL != "" {
		repo, err := s.opts.Store.Repositories.GetByURL(r.Context(), req.Source.RepoURL)
		if err != nil {
			writeError(w, http.StatusBadRequest, "unknown_repo", err.Error())
			return
		}
		if err := s.enforceProjectSourceAndDest(r.Context(), app.Project, req.Source.RepoURL, nil, ""); err != nil {
			writeError(w, http.StatusForbidden, "project_policy", err.Error())
			return
		}
		app.RepoID = repo.ID
	}
	if req.Source.Path != "" {
		app.Path = req.Source.Path
	}
	if req.Source.TargetRevision != "" {
		app.TargetRevision = req.Source.TargetRevision
	}
	app.HelmValueFiles = append([]string(nil), req.Source.HelmValueFiles...)
	if req.Destination.Cluster != "" {
		cl, err := s.opts.Store.Clusters.GetByName(r.Context(), req.Destination.Cluster)
		if err != nil {
			writeError(w, http.StatusBadRequest, "unknown_cluster", err.Error())
			return
		}
		app.DestClusterID = cl.ID
	}
	if req.Destination.Namespace != "" {
		app.DestNamespace = req.Destination.Namespace
	}
	if req.SyncPolicy.Automated != nil {
		app.SyncPolicy.Automated = &domain.AutomatedSync{
			Prune:    req.SyncPolicy.Automated.Prune,
			SelfHeal: req.SyncPolicy.Automated.SelfHeal,
		}
	}
	app.SyncPolicy.SyncOptions = append([]string(nil), req.SyncPolicy.SyncOptions...)
	if m := req.SyncPolicy.ManagedNamespaceMetadata; m != nil {
		app.SyncPolicy.ManagedNamespaceMetadata = &domain.ManagedNamespaceMetadata{
			Labels:      maps.Clone(m.Labels),
			Annotations: maps.Clone(m.Annotations),
		}
	} else {
		app.SyncPolicy.ManagedNamespaceMetadata = nil
	}
	app.SyncPolicy.CreateNamespace = req.SyncPolicy.CreateNamespace
	app.SyncPolicy.IgnoreDifferences = apiIgnoreDiffToDomain(req.SyncPolicy.IgnoreDifferences)
	if err := applyHelmValuesJSON(app, req.Source.HelmValues); err != nil {
		if he, ok := err.(*httpCreateErr); ok {
			writeError(w, he.code, he.key, he.msg)
			return
		}
		writeError(w, http.StatusInternalServerError, "helm_values", err.Error())
		return
	}
	if err := s.opts.Store.Applications.Update(r.Context(), app); err != nil {
		writeError(w, http.StatusInternalServerError, "update_failed", err.Error())
		return
	}
	s.opts.Controller.EnqueueStatus(app.Name)
	dto, _ := s.toAPIApp(r.Context(), app)
	writeJSON(w, http.StatusOK, dto)
}

func (s *Server) deleteApplication(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	app, err := s.opts.Store.Applications.GetByName(r.Context(), name)
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	if !requireProjectAccess(w, r, app.Project) {
		return
	}
	if err := s.opts.Store.Applications.Delete(r.Context(), app.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "delete_failed", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) syncApplication(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	app, err := s.opts.Store.Applications.GetByName(r.Context(), name)
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	if !requireProjectAccess(w, r, app.Project) {
		return
	}
	if s.opts.Config.SyncDeniedAt(time.Now()) {
		writeError(w, http.StatusLocked, "sync_window", "sync is denied during the configured maintenance window (SYNC_DENY_RANGE_UTC)")
		return
	}
	var req apiv1.SyncRequest
	_ = decodeJSON(r, &req)
	revision := req.Revision
	if revision == "" {
		revision = app.TargetRevision
	}
	user, _ := auth.FromContext(r.Context())
	op := &domain.SyncOperation{
		AppID:       app.ID,
		Revision:    revision,
		InitiatedBy: user.Subject,
		Status:      domain.SyncOpPending,
		Request: domain.SyncRunRequest{
			DryRun:    req.DryRun,
			Prune:     req.Prune,
			Resources: req.Resources,
		},
	}
	if err := s.opts.Store.Sync.Create(r.Context(), op); err != nil {
		writeError(w, http.StatusInternalServerError, "sync_failed", err.Error())
		return
	}
	s.opts.Controller.EnqueueSync(app.Name)
	writeJSON(w, http.StatusAccepted, map[string]string{"syncId": op.ID, "status": string(op.Status)})
}

func (s *Server) refreshApplication(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if _, ok := s.appByNameAuthorized(w, r, name); !ok {
		return
	}
	s.opts.Controller.EnqueueStatus(name)
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) appManifests(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	app, ok := s.appByNameAuthorized(w, r, name)
	if !ok {
		return
	}
	res, err := s.opts.Repo.RenderForApp(r.Context(), app)
	if err != nil {
		writeError(w, http.StatusBadGateway, "render_failed", err.Error())
		return
	}
	applicable := manifest.FilterApplicable(res.Objects)
	out := make([]map[string]any, 0, len(applicable))
	for _, o := range applicable {
		out = append(out, o.Object)
	}
	writeJSON(w, http.StatusOK, map[string]any{"revision": res.Revision, "manifests": out})
}

func (s *Server) appDiff(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	app, ok := s.appByNameAuthorized(w, r, name)
	if !ok {
		return
	}
	res, err := s.opts.Repo.RenderForApp(r.Context(), app)
	if err != nil {
		writeError(w, http.StatusBadGateway, "render_failed", err.Error())
		return
	}
	applicable := manifest.FilterApplicable(res.Objects)
	live, _, err := collectLiveForAPI(r, s, app, applicable)
	if err != nil {
		writeError(w, http.StatusBadGateway, "live_failed", err.Error())
		return
	}
	ds, err := k8s.Diff(applicable, live, app.SyncPolicy.IgnoreDifferences)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "diff_failed", err.Error())
		return
	}
	resp := apiv1.DiffResponse{OutOfSync: ds.OutOfSync, Synced: ds.Synced}
	for _, it := range ds.Items {
		sync := "OutOfSync"
		if it.Synced {
			sync = "Synced"
		}
		resp.Resources = append(resp.Resources, apiv1.ResourceDiff{
			Group:          it.Group,
			Version:        it.Version,
			Kind:           it.Kind,
			Namespace:      it.Namespace,
			Name:           it.Name,
			Sync:           sync,
			DesiredYAML:    it.DesiredYAML,
			LiveYAML:       it.LiveYAML,
			NormalizedDiff: it.UnifiedDiff,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) appResourceTree(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	app, ok := s.appByNameAuthorized(w, r, name)
	if !ok {
		return
	}
	nodes, err := s.opts.Cluster.BuildTree(r.Context(), app.Name, app.DestNamespace)
	if err != nil {
		writeError(w, http.StatusBadGateway, "tree_failed", err.Error())
		return
	}
	out := apiv1.ResourceTree{}
	for _, n := range nodes {
		out.Nodes = append(out.Nodes, toAPINode(n))
	}

	// Append child applications (App of Apps) as synthetic ResourceNodes so
	// they appear in the topology even though they are not live k8s objects.
	childApps, _ := s.opts.Store.Applications.ListByParent(r.Context(), app.Name)
	for _, child := range childApps {
		childStatus, _ := s.opts.Store.Status.Get(r.Context(), child.ID)
		health := "Unknown"
		sync := "Unknown"
		if childStatus != nil {
			health = string(childStatus.HealthStatus)
			sync = string(childStatus.SyncStatus)
		}
		out.Nodes = append(out.Nodes, apiv1.ResourceNode{
			Group:   "k8s-ui.io",
			Version: "v1",
			Kind:    "Application",
			Name:    child.Name,
			UID:     child.ID,
			Health:  health,
			Sync:    sync,
			CreationTimestamp: child.CreatedAt.UTC().Format(time.RFC3339),
		})
	}

	s.enrichResourceTree(r, app, &out)
	writeJSON(w, http.StatusOK, out)
}

func toAPINode(n *k8s.ResourceNode) apiv1.ResourceNode {
	out := apiv1.ResourceNode{
		Group:     n.Object.GroupVersionKind().Group,
		Version:   n.Object.GroupVersionKind().Version,
		Kind:      n.Object.GetKind(),
		Namespace: n.Object.GetNamespace(),
		Name:      n.Object.GetName(),
		UID:       string(n.Object.GetUID()),
		Health:    string(k8s.Health(n.Object)),
		Sync:      "Synced",
	}
	if ts := n.Object.GetCreationTimestamp(); !ts.IsZero() {
		out.CreationTimestamp = ts.UTC().Format(time.RFC3339)
	}
	out.Labels = n.Object.GetLabels()
	if out.Kind == "Pod" {
		if phase, _, _ := unstructured.NestedString(n.Object.Object, "status", "phase"); phase != "" {
			out.PodPhase = phase
		}
	}
	for _, c := range n.Children {
		out.Children = append(out.Children, toAPINode(c))
	}
	return out
}

func (s *Server) appHistory(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	app, ok := s.appByNameAuthorized(w, r, name)
	if !ok {
		return
	}
	ops, err := s.opts.Store.Sync.ListByApp(r.Context(), app.ID, 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "history_failed", err.Error())
		return
	}
	out := make([]apiv1.SyncOperation, 0, len(ops))
	for _, op := range ops {
		out = append(out, apiv1.SyncOperation{
			ID:          op.ID,
			AppName:     app.Name,
			Revision:    op.Revision,
			StartedAt:   op.StartedAt,
			FinishedAt:  op.FinishedAt,
			Status:      string(op.Status),
			InitiatedBy: op.InitiatedBy,
			Message:     op.Message,
			Resources:   toAPISyncResults(op.Resources),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// appByNameAuthorized loads an application and enforces project scope.
func (s *Server) appByNameAuthorized(w http.ResponseWriter, r *http.Request, name string) (*domain.Application, bool) {
	app, err := s.opts.Store.Applications.GetByName(r.Context(), name)
	if err != nil {
		notFoundOr500(w, err)
		return nil, false
	}
	if !requireProjectAccess(w, r, app.Project) {
		return nil, false
	}
	return app, true
}

func toAPISyncResults(in []domain.SyncResourceResult) []apiv1.SyncResourceResult {
	out := make([]apiv1.SyncResourceResult, 0, len(in))
	for _, r := range in {
		out = append(out, apiv1.SyncResourceResult{
			Group:     r.Group,
			Version:   r.Version,
			Kind:      r.Kind,
			Namespace: r.Namespace,
			Name:      r.Name,
			Status:    r.Status,
			Message:   r.Message,
		})
	}
	return out
}

// deleteLiveResource deletes a live cluster resource identified by query params:
// group, version, kind, namespace, name.
func (s *Server) deleteLiveResource(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if _, ok := s.appByNameAuthorized(w, r, name); !ok {
		return
	}
	q := r.URL.Query()
	kind := q.Get("kind")
	resName := q.Get("name")
	namespace := q.Get("namespace")
	group := q.Get("group")
	version := q.Get("version")
	if kind == "" || resName == "" || version == "" {
		writeError(w, http.StatusBadRequest, "missing_params", "kind, version and name query params are required")
		return
	}
	gvk := schema.GroupVersionKind{Group: group, Version: version, Kind: kind}
	if err := s.opts.Cluster.Delete(r.Context(), gvk, namespace, resName); err != nil {
		writeError(w, http.StatusBadGateway, "delete_failed", err.Error())
		return
	}
	s.opts.Controller.EnqueueStatus(name)
	w.WriteHeader(http.StatusNoContent)
}

// syncLiveResource triggers a targeted sync for a single resource identified by
// query params: group, version, kind, namespace, name.
func (s *Server) syncLiveResource(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	app, ok := s.appByNameAuthorized(w, r, name)
	if !ok {
		return
	}
	if s.opts.Config.SyncDeniedAt(time.Now()) {
		writeError(w, http.StatusLocked, "sync_window", "sync is denied during the configured maintenance window")
		return
	}
	q := r.URL.Query()
	kind := q.Get("kind")
	resName := q.Get("name")
	namespace := q.Get("namespace")
	group := q.Get("group")
	version := q.Get("version")
	if kind == "" || resName == "" || version == "" {
		writeError(w, http.StatusBadRequest, "missing_params", "kind, version and name query params are required")
		return
	}
	resourceKey := group + "/" + version + "/" + kind + "/" + namespace + "/" + resName
	user, _ := auth.FromContext(r.Context())
	op := &domain.SyncOperation{
		AppID:       app.ID,
		Revision:    app.TargetRevision,
		InitiatedBy: user.Subject,
		Status:      domain.SyncOpPending,
		Request: domain.SyncRunRequest{
			Resources: []string{resourceKey},
		},
	}
	if err := s.opts.Store.Sync.Create(r.Context(), op); err != nil {
		writeError(w, http.StatusInternalServerError, "sync_failed", err.Error())
		return
	}
	s.opts.Controller.EnqueueSync(app.Name)
	writeJSON(w, http.StatusAccepted, map[string]string{"syncId": op.ID, "status": string(op.Status)})
}

// applyLiveResource accepts a YAML body and applies it to the live cluster via SSA.
// This implements the "live manifest edit" feature: the user edits desired YAML in
// the browser and the change is applied directly to Kubernetes (not to git).
func (s *Server) applyLiveResource(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	if _, ok := s.appByNameAuthorized(w, r, name); !ok {
		return
	}
	body, err := func() ([]byte, error) {
		defer r.Body.Close()
		b := make([]byte, 0, 64*1024)
		buf := bytes.NewBuffer(b)
		if _, e := buf.ReadFrom(r.Body); e != nil {
			return nil, e
		}
		return buf.Bytes(), nil
	}()
	if err != nil {
		writeError(w, http.StatusBadRequest, "read_body", err.Error())
		return
	}
	if len(bytes.TrimSpace(body)) == 0 {
		writeError(w, http.StatusBadRequest, "empty_body", "YAML body is required")
		return
	}
	obj, err := manifest.ParseYAMLToUnstructured(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_yaml", err.Error())
		return
	}
	result, err := s.opts.Cluster.Apply(r.Context(), obj, k8s.ApplyOptions{Force: true})
	if err != nil {
		writeError(w, http.StatusBadGateway, "apply_failed", err.Error())
		return
	}
	s.opts.Controller.EnqueueStatus(name)
	writeJSON(w, http.StatusOK, result.Object)
}
