package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/orin/orin/internal/domain"
	"github.com/orin/orin/internal/store"
)

// --- API DTOs ---------------------------------------------------------------

type pluginGenerateDTO struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
}

type envVarDTO struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type pluginDTO struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Generate pluginGenerateDTO `json:"generate"`
	Env      []envVarDTO       `json:"env,omitempty"`
}

type createPluginRequest struct {
	Name     string            `json:"name"`
	Generate pluginGenerateDTO `json:"generate"`
	Env      []envVarDTO       `json:"env,omitempty"`
}

type updatePluginRequest struct {
	Generate pluginGenerateDTO `json:"generate"`
	Env      []envVarDTO       `json:"env,omitempty"`
}

// --- helpers ----------------------------------------------------------------

func domainEnvVars(dtos []envVarDTO) []domain.EnvVar {
	out := make([]domain.EnvVar, 0, len(dtos))
	for _, d := range dtos {
		out = append(out, domain.EnvVar{Name: d.Name, Value: d.Value})
	}
	return out
}

func toEnvVarDTOs(envs []domain.EnvVar) []envVarDTO {
	out := make([]envVarDTO, 0, len(envs))
	for _, e := range envs {
		out = append(out, envVarDTO{Name: e.Name, Value: e.Value})
	}
	return out
}

func toPluginDTO(p *domain.Plugin) pluginDTO {
	return pluginDTO{
		ID:   p.ID,
		Name: p.Name,
		Generate: pluginGenerateDTO{
			Command: p.Generate.Command,
			Args:    append([]string(nil), p.Generate.Args...),
		},
		Env: toEnvVarDTOs(p.Env),
	}
}

// --- handlers ---------------------------------------------------------------

func (s *Server) listPlugins(w http.ResponseWriter, r *http.Request) {
	plugins, err := s.opts.Store.Plugins.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	out := make([]pluginDTO, 0, len(plugins))
	for _, p := range plugins {
		out = append(out, toPluginDTO(p))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) createPlugin(w http.ResponseWriter, r *http.Request) {
	var req createPluginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "name is required")
		return
	}
	if req.Generate.Command == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "generate.command is required")
		return
	}
	plugin := &domain.Plugin{
		Name: req.Name,
		Generate: domain.PluginGenerateSpec{
			Command: req.Generate.Command,
			Args:    req.Generate.Args,
		},
		Env: domainEnvVars(req.Env),
	}
	if err := s.opts.Store.Plugins.Create(r.Context(), plugin); err != nil {
		writeError(w, http.StatusInternalServerError, "create_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, toPluginDTO(plugin))
}

func (s *Server) getPlugin(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	plugin, err := s.opts.Store.Plugins.GetByID(r.Context(), id)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "not_found", "plugin not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "get_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toPluginDTO(plugin))
}

func (s *Server) updatePlugin(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	plugin, err := s.opts.Store.Plugins.GetByID(r.Context(), id)
	if err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "not_found", "plugin not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "get_failed", err.Error())
		return
	}

	var req updatePluginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}
	if req.Generate.Command == "" {
		writeError(w, http.StatusBadRequest, "validation_error", "generate.command is required")
		return
	}

	plugin.Generate = domain.PluginGenerateSpec{
		Command: req.Generate.Command,
		Args:    req.Generate.Args,
	}
	plugin.Env = domainEnvVars(req.Env)

	if err := s.opts.Store.Plugins.Update(r.Context(), plugin); err != nil {
		writeError(w, http.StatusInternalServerError, "update_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toPluginDTO(plugin))
}

func (s *Server) deletePlugin(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.opts.Store.Plugins.Delete(r.Context(), id); err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "not_found", "plugin not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "delete_failed", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
