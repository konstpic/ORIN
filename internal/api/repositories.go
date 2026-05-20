package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/orin/orin/internal/domain"
	"github.com/orin/orin/internal/rbac"
	"github.com/orin/orin/internal/rbacenforce"
	"github.com/orin/orin/internal/reposerver"
	apiv1 "github.com/orin/orin/pkg/api/v1"
)

func (s *Server) listRepositories(w http.ResponseWriter, r *http.Request) {
	repos, err := s.opts.Store.Repositories.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	out := make([]apiv1.Repository, 0, len(repos))
	for _, repo := range repos {
		out = append(out, apiv1.Repository{
			ID:        repo.ID,
			URL:       repo.URL,
			Type:      string(repo.Type),
			HasCreds:  len(repo.CredentialsEncrypted) > 0,
			CreatedAt: repo.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) createRepository(w http.ResponseWriter, r *http.Request) {
	if !rbacenforce.CheckPermission(w, r, rbac.PermRepoCreate) {
		return
	}
	var req apiv1.CreateRepositoryRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if req.URL == "" {
		writeError(w, http.StatusBadRequest, "missing_url", "url is required")
		return
	}
	repo := &domain.Repository{URL: req.URL, Type: domain.RepoTypeGit}
	if req.Username != "" || req.Password != "" {
		b, err := reposerver.EncodeCreds(&domain.RepoCreds{Username: req.Username, Password: req.Password})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "encode_creds", err.Error())
			return
		}
		enc, err := s.opts.Cipher.Encrypt(b)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "encrypt_creds", err.Error())
			return
		}
		repo.CredentialsEncrypted = enc
	}
	if err := s.opts.Store.Repositories.Create(r.Context(), repo); err != nil {
		writeError(w, http.StatusInternalServerError, "create_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, apiv1.Repository{
		ID:        repo.ID,
		URL:       repo.URL,
		Type:      string(repo.Type),
		HasCreds:  len(repo.CredentialsEncrypted) > 0,
		CreatedAt: repo.CreatedAt,
	})
}

func (s *Server) deleteRepository(w http.ResponseWriter, r *http.Request) {
	if !rbacenforce.CheckPermission(w, r, rbac.PermRepoDelete) {
		return
	}
	id := chi.URLParam(r, "id")
	if err := s.opts.Store.Repositories.Delete(r.Context(), id); err != nil {
		notFoundOr500(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listClusters(w http.ResponseWriter, r *http.Request) {
	cls, err := s.opts.Store.Clusters.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	out := make([]apiv1.Cluster, 0, len(cls))
	for _, cl := range cls {
		out = append(out, apiv1.Cluster{
			ID:        cl.ID,
			Name:      cl.Name,
			ServerURL: cl.ServerURL,
			InCluster: cl.InCluster,
			CreatedAt: cl.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, out)
}
