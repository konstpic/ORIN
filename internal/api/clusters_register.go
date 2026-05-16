package api

import (
	"net/http"

	"github.com/k8s-ui/k8s-ui/internal/domain"
	"github.com/k8s-ui/k8s-ui/internal/k8s"
	"github.com/k8s-ui/k8s-ui/internal/rbac"
	"github.com/k8s-ui/k8s-ui/internal/rbacenforce"
	apiv1 "github.com/k8s-ui/k8s-ui/pkg/api/v1"
)

func (s *Server) createCluster(w http.ResponseWriter, r *http.Request) {
	if !rbacenforce.CheckPermission(w, r, rbac.PermClusterCreate) {
		return
	}
	var req apiv1.CreateClusterRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if req.Name == "" || req.KubeconfigYAML == "" {
		writeError(w, http.StatusBadRequest, "missing_fields", "name and kubeconfigYaml are required")
		return
	}
	if req.Name == "in-cluster" {
		writeError(w, http.StatusBadRequest, "reserved_name", "cluster name in-cluster is reserved")
		return
	}
	rc, err := k8s.RESTConfigFromKubeconfigYAML(req.KubeconfigYAML)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_kubeconfig", err.Error())
		return
	}
	enc, err := s.opts.Cipher.Encrypt([]byte(req.KubeconfigYAML))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "encrypt_failed", err.Error())
		return
	}
	cl := &domain.Cluster{
		Name:                req.Name,
		ServerURL:           rc.Host,
		AuthConfigEncrypted: enc,
		InCluster:           false,
	}
	if err := s.opts.Store.Clusters.Upsert(r.Context(), cl); err != nil {
		writeError(w, http.StatusInternalServerError, "upsert_failed", err.Error())
		return
	}
	row, err := s.opts.Store.Clusters.GetByName(r.Context(), req.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "load_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, apiv1.Cluster{
		ID:        row.ID,
		Name:      row.Name,
		ServerURL: row.ServerURL,
		InCluster: row.InCluster,
		CreatedAt: row.CreatedAt,
	})
}
