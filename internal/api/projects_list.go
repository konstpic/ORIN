package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/k8s-ui/k8s-ui/internal/domain"
	apiv1 "github.com/k8s-ui/k8s-ui/pkg/api/v1"
)

func (s *Server) listProjects(w http.ResponseWriter, r *http.Request) {
	rows, err := s.opts.Store.Projects.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	out := make([]apiv1.Project, 0, len(rows))
	for _, p := range rows {
		out = append(out, domainProjectToAPI(p))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) createProject(w http.ResponseWriter, r *http.Request) {
	var req apiv1.CreateProjectRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "missing_name", "name is required")
		return
	}
	pr := &domain.Project{
		Name:        req.Name,
		Description: req.Description,
		Policies:    apiPoliciesToDomain(req.Policies),
	}
	if err := s.opts.Store.Projects.Create(r.Context(), pr); err != nil {
		writeError(w, http.StatusInternalServerError, "create_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, domainProjectToAPI(*pr))
}

func (s *Server) updateProject(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	pr, err := s.opts.Store.Projects.GetByName(r.Context(), name)
	if err != nil {
		notFoundOr500(w, err)
		return
	}
	var req apiv1.UpdateProjectRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	pr.Description = req.Description
	pr.Policies = apiPoliciesToDomain(req.Policies)
	if err := s.opts.Store.Projects.Update(r.Context(), pr); err != nil {
		writeError(w, http.StatusInternalServerError, "update_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, domainProjectToAPI(*pr))
}

func domainProjectToAPI(p domain.Project) apiv1.Project {
	out := apiv1.Project{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		CreatedAt:   p.CreatedAt,
	}
	pol := p.Policies
	out.Policies.SourceRepos = append([]string(nil), pol.SourceRepos...)
	for _, d := range pol.Destinations {
		out.Policies.Destinations = append(out.Policies.Destinations, apiv1.ProjectDestination{
			Server:    d.Server,
			Name:      d.Name,
			Namespace: d.Namespace,
		})
	}
	for _, r := range pol.ClusterResourceWhitelist {
		out.Policies.ClusterResourceWhitelist = append(out.Policies.ClusterResourceWhitelist, apiv1.ProjectResourceRule{Group: r.Group, Kind: r.Kind})
	}
	for _, r := range pol.NamespaceResourceBlacklist {
		out.Policies.NamespaceResourceBlacklist = append(out.Policies.NamespaceResourceBlacklist, apiv1.ProjectResourceRule{Group: r.Group, Kind: r.Kind})
	}
	return out
}

func apiPoliciesToDomain(p apiv1.ProjectPolicies) domain.ProjectPolicies {
	out := domain.ProjectPolicies{
		SourceRepos: append([]string(nil), p.SourceRepos...),
	}
	for _, d := range p.Destinations {
		out.Destinations = append(out.Destinations, domain.ProjectDestination{
			Server:    d.Server,
			Name:      d.Name,
			Namespace: d.Namespace,
		})
	}
	for _, r := range p.ClusterResourceWhitelist {
		out.ClusterResourceWhitelist = append(out.ClusterResourceWhitelist, domain.ProjectResourceRule{Group: r.Group, Kind: r.Kind})
	}
	for _, r := range p.NamespaceResourceBlacklist {
		out.NamespaceResourceBlacklist = append(out.NamespaceResourceBlacklist, domain.ProjectResourceRule{Group: r.Group, Kind: r.Kind})
	}
	return out
}

