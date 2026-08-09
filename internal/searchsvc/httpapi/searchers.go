package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/rinat1313/zakupki-parser/internal/searchsvc/db"
	"github.com/rinat1313/zakupki-parser/internal/searchsvc/models"
)

func (s *Server) handleListSearchers(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	list, err := s.Store.ListSearchers(r.Context(), u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list failed")
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) handleGetSearcher(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	item, err := s.Store.GetSearcher(r.Context(), u.ID, r.PathValue("id"))
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get failed")
		return
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) handleCreateSearcher(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	var req models.SearcherWrite
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}
	cfg := models.DefaultSearcherConfig()
	if req.Config != nil {
		cfg = *req.Config
	}
	autoAI := false
	if req.AutoAI != nil {
		autoAI = *req.AutoAI
	}

	item, err := s.Store.CreateSearcher(r.Context(), u.ID, name, cfg, autoAI)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "searcher name already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "create failed")
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (s *Server) handleUpdateSearcher(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	id := r.PathValue("id")
	existing, err := s.Store.GetSearcher(r.Context(), u.ID, id)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get failed")
		return
	}

	var req models.SearcherWrite
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = existing.Name
	}
	cfg := existing.Config
	if req.Config != nil {
		cfg = *req.Config
	}
	autoAI := existing.AutoAI
	if req.AutoAI != nil {
		autoAI = *req.AutoAI
	}

	out, err := s.Store.UpdateSearcher(r.Context(), u.ID, id, name, cfg, autoAI)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "searcher name already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "update failed")
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleDeleteSearcher(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	if err := s.Store.DeleteSearcher(r.Context(), u.ID, r.PathValue("id")); errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "delete failed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleSetAutoAI(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	out, err := s.Store.SetSearcherAutoAI(r.Context(), u.ID, r.PathValue("id"), req.Enabled)
	if errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "update failed")
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleRunSearcher(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	id := r.PathValue("id")
	if _, err := s.Store.GetSearcher(r.Context(), u.ID, id); errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "get failed")
		return
	}

	run, err := s.Runner.RunSearcher(r.Context(), u.ID, id)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, run)
}

func (s *Server) handleListTenders(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	id := r.PathValue("id")
	if _, err := s.Store.GetSearcher(r.Context(), u.ID, id); errors.Is(err, db.ErrNotFound) {
		writeError(w, http.StatusNotFound, "not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "get failed")
		return
	}

	list, err := s.Store.ListSearcherTenders(r.Context(), id, r.URL.Query().Get("q"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list failed")
		return
	}
	list = s.Runner.EnrichTendersFromCore(r.Context(), list)
	writeJSON(w, http.StatusOK, list)
}
