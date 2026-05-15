package controller

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/k8s-ui/k8s-ui/internal/appcatalog"
	"github.com/k8s-ui/k8s-ui/internal/store"
)

func (c *Controller) reconcileAppsCatalog(ctx context.Context) {
	cfg := c.cfg
	if cfg.AppsCatalogRepoURL == "" {
		return
	}
	rctx, cancel := context.WithTimeout(ctx, cfg.RepoRenderTimeout)
	defer cancel()

	raw, err := c.repo.ReadRawFile(rctx, cfg.AppsCatalogRepoURL, cfg.AppsCatalogRevision, cfg.AppsCatalogPath)
	if err != nil {
		slog.Warn("apps catalog: read file", "err", err)
		return
	}

	resolve := c.argoDestinationResolver(ctx)
	appEntries, projEntries, err := appcatalog.ParseCatalogYAML(raw, resolve)
	if err != nil {
		slog.Warn("apps catalog: parse yaml", "err", err)
		return
	}

	// Upsert projects before applications so that project policies are
	// enforced from the first reconcile of any new application.
	for _, e := range projEntries {
		want, err := appcatalog.DomainProjectFromEntry(e)
		if err != nil {
			slog.Warn("apps catalog: bad project entry", "name", e.Name, "err", err)
			continue
		}
		cur, err := c.store.Projects.GetByName(ctx, want.Name)
		if err != nil {
			if !errors.Is(err, store.ErrNotFound) {
				slog.Warn("apps catalog: get project", "name", want.Name, "err", err)
				continue
			}
			if err := c.store.Projects.Create(ctx, want); err != nil {
				slog.Warn("apps catalog: create project", "name", want.Name, "err", err)
				continue
			}
			slog.Info("apps catalog: created project", "name", want.Name)
			continue
		}
		want.ID = cur.ID
		if !appcatalog.ProjectNeedsDBUpdate(cur, want) {
			continue
		}
		if err := c.store.Projects.Update(ctx, want); err != nil {
			slog.Warn("apps catalog: update project", "name", want.Name, "err", err)
			continue
		}
		slog.Info("apps catalog: updated project", "name", want.Name)
	}

	for _, e := range appEntries {
		want, err := appcatalog.DomainFromEntry(ctx, c.store, e)
		if err != nil {
			slog.Warn("apps catalog: bad entry", "name", e.Name, "err", err)
			continue
		}
		cur, err := c.store.Applications.GetByName(ctx, want.Name)
		if err != nil {
			if !errors.Is(err, store.ErrNotFound) {
				slog.Warn("apps catalog: get app", "name", want.Name, "err", err)
				continue
			}
			if err := c.store.Applications.Create(ctx, want); err != nil {
				slog.Warn("apps catalog: create", "name", want.Name, "err", err)
				continue
			}
			slog.Info("apps catalog: created application", "name", want.Name)
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
		if !appcatalog.NeedsDBUpdate(cur, &up) {
			continue
		}
		if err := c.store.Applications.Update(ctx, &up); err != nil {
			slog.Warn("apps catalog: update", "name", want.Name, "err", err)
			continue
		}
		slog.Info("apps catalog: updated application", "name", want.Name)
		c.EnqueueStatus(want.Name)
	}
}

func (c *Controller) appsCatalogTicker(ctx context.Context) {
	if c.cfg.AppsCatalogRepoURL == "" {
		return
	}
	c.reconcileAppsCatalog(ctx)
	t := time.NewTicker(c.cfg.AppsCatalogInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			c.reconcileAppsCatalog(ctx)
		}
	}
}
