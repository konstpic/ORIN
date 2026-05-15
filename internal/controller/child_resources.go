package controller

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/k8s-ui/k8s-ui/internal/appcatalog"
	"github.com/k8s-ui/k8s-ui/internal/domain"
	"github.com/k8s-ui/k8s-ui/internal/store"
)

// reconcileChildResources scans rendered manifests of the parent Application for
// k8s-ui.io/Application, k8s-ui.io/AppProject, argoproj.io/Application, and
// argoproj.io/AppProject objects and upserts them into the database.
//
// This is the "app of apps" flow: a parent chart declares child applications or
// projects as first-class objects; k8s-ui reads them and creates/updates the
// corresponding database rows.  The objects themselves are never applied to the
// destination cluster (see manifest.IsControlPlaneObject).
//
// Projects are reconciled before Applications so that per-app project policies
// are enforced from the first reconcile.
func (c *Controller) reconcileChildResources(ctx context.Context, parent *domain.Application, rendered []*unstructured.Unstructured) {
	resolve := c.argoDestinationResolver(ctx)

	var appEntries []appcatalog.Entry
	var projEntries []appcatalog.ProjectEntry

	for _, u := range rendered {
		appEntry, projEntry, kind, ok, err := appcatalog.TryEntryFromObject(u, resolve)
		if err != nil {
			slog.Warn("child resources: skipping invalid object",
				"parent", parent.Name,
				"resource", u.GetName(),
				"kind", u.GetKind(),
				"apiVersion", u.GetAPIVersion(),
				"err", err,
			)
			continue
		}
		if !ok {
			continue
		}
		switch kind {
		case appcatalog.EntryKindApplication:
			appEntries = append(appEntries, appEntry)
		case appcatalog.EntryKindAppProject:
			projEntries = append(projEntries, projEntry)
		}
	}

	c.upsertChildProjects(ctx, parent, projEntries)
	c.upsertChildApps(ctx, parent, appEntries)
}

func (c *Controller) upsertChildProjects(ctx context.Context, parent *domain.Application, entries []appcatalog.ProjectEntry) {
	for _, e := range entries {
		want, err := appcatalog.DomainProjectFromEntry(e)
		if err != nil {
			slog.Warn("child resources: bad project entry", "parent", parent.Name, "project", e.Name, "err", err)
			continue
		}
		cur, err := c.store.Projects.GetByName(ctx, want.Name)
		if err != nil {
			if !errors.Is(err, store.ErrNotFound) {
				slog.Warn("child resources: get project", "name", want.Name, "err", err)
				continue
			}
			if err := c.store.Projects.Create(ctx, want); err != nil {
				slog.Warn("child resources: create project", "name", want.Name, "err", err)
				continue
			}
			slog.Info("child resources: created project", "parent", parent.Name, "project", want.Name)
			continue
		}
		want.ID = cur.ID
		if !appcatalog.ProjectNeedsDBUpdate(cur, want) {
			continue
		}
		if err := c.store.Projects.Update(ctx, want); err != nil {
			slog.Warn("child resources: update project", "name", want.Name, "err", err)
			continue
		}
		slog.Info("child resources: updated project", "parent", parent.Name, "project", want.Name)
	}
}

func (c *Controller) upsertChildApps(ctx context.Context, parent *domain.Application, entries []appcatalog.Entry) {
	for _, e := range entries {
		if e.Name == parent.Name {
			slog.Warn("child resources: skip entry with same name as parent", "name", e.Name)
			continue
		}
		want, err := appcatalog.DomainFromEntry(ctx, c.store, e)
		if err != nil {
			slog.Warn("child resources: bad app entry", "parent", parent.Name, "child", e.Name, "err", err)
			continue
		}
		want.ParentApp = parent.Name
		cur, err := c.store.Applications.GetByName(ctx, want.Name)
		if err != nil {
			if !errors.Is(err, store.ErrNotFound) {
				slog.Warn("child resources: get app", "name", want.Name, "err", err)
				continue
			}
			if err := c.store.Applications.Create(ctx, want); err != nil {
				slog.Warn("child resources: create app", "name", want.Name, "err", err)
				continue
			}
			slog.Info("child resources: created app", "parent", parent.Name, "child", want.Name)
			c.EnqueueStatus(want.Name)
			continue
		}
		up := *cur
		up.RepoID = want.RepoID
		up.Path = want.Path
		up.TargetRevision = want.TargetRevision
		up.DestClusterID = want.DestClusterID
		up.DestNamespace = want.DestNamespace
		up.HelmValuesJSON = want.HelmValuesJSON
		up.HelmValueFiles = want.HelmValueFiles
		up.SyncPolicy = want.SyncPolicy
		up.ParentApp = parent.Name
		if !appcatalog.NeedsDBUpdate(cur, &up) {
			continue
		}
		if err := c.store.Applications.Update(ctx, &up); err != nil {
			slog.Warn("child resources: update app", "name", want.Name, "err", err)
			continue
		}
		slog.Info("child resources: updated app", "parent", parent.Name, "child", want.Name)
		c.EnqueueStatus(want.Name)
	}
}

func (c *Controller) argoDestinationResolver(ctx context.Context) appcatalog.ArgoDestinationResolve {
	return func(server, name string) (string, error) {
		name = strings.TrimSpace(name)
		server = strings.TrimSpace(server)
		if name != "" {
			if _, err := c.store.Clusters.GetByName(ctx, name); err == nil {
				return name, nil
			}
			return "", fmt.Errorf("unknown cluster name %q (destination.name)", name)
		}
		if server == "" {
			return "", fmt.Errorf("destination needs server or name")
		}
		serverNorm := strings.TrimSuffix(server, "/")
		clusters, err := c.store.Clusters.List(ctx)
		if err != nil {
			return "", err
		}
		for _, cl := range clusters {
			if strings.TrimSuffix(cl.ServerURL, "/") == serverNorm {
				return cl.Name, nil
			}
		}
		if strings.Contains(serverNorm, "kubernetes.default") {
			if _, err := c.store.Clusters.GetByName(ctx, "in-cluster"); err == nil {
				return "in-cluster", nil
			}
		}
		return "", fmt.Errorf("no registered cluster for destination.server %q", server)
	}
}
