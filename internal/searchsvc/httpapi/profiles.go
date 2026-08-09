package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/rinat1313/zakupki-parser/internal/searchsvc/db"
	"github.com/rinat1313/zakupki-parser/internal/searchsvc/models"
)

// Legacy search-profiles shape for older clients.
type legacyProfile struct {
	ID        string                `json:"id"`
	UserID    string                `json:"user_id,omitempty"`
	Name      string                `json:"name"`
	Source    string                `json:"source"`
	EISConfig models.SearcherConfig `json:"eis_config"`
	Enabled   bool                  `json:"enabled"`
	AutoAI    bool                  `json:"auto_ai"`
	CreatedAt any                   `json:"created_at"`
	UpdatedAt any                   `json:"updated_at"`
}

func searcherToProfile(s models.Searcher) legacyProfile {
	return legacyProfile{
		ID:        s.ID,
		UserID:    s.UserID,
		Name:      s.Name,
		Source:    "eis",
		EISConfig: s.Config,
		Enabled:   true,
		AutoAI:    s.AutoAI,
		CreatedAt: s.CreatedAt,
		UpdatedAt: s.UpdatedAt,
	}
}

func (s *Server) handleListProfiles(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	list, err := s.Store.ListSearchers(r.Context(), u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "list failed")
		return
	}
	items := make([]legacyProfile, 0, len(list))
	for _, item := range list {
		items = append(items, searcherToProfile(item))
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (s *Server) handleGetProfile(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, http.StatusOK, searcherToProfile(item))
}

type legacyProfileWrite struct {
	Name      string                 `json:"name"`
	EISConfig *models.SearcherConfig `json:"eis_config"`
	Config    *models.SearcherConfig `json:"config"`
	AutoAI    *bool                  `json:"auto_ai"`
	Enabled   *bool                  `json:"enabled"`
}

func (s *Server) handleCreateProfile(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	var req legacyProfileWrite
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
	if req.EISConfig != nil {
		cfg = *req.EISConfig
	} else if req.Config != nil {
		cfg = *req.Config
	}
	autoAI := false
	if req.AutoAI != nil {
		autoAI = *req.AutoAI
	}
	item, err := s.Store.CreateSearcher(r.Context(), u.ID, name, cfg, autoAI)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "profile name already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "create failed")
		return
	}
	writeJSON(w, http.StatusCreated, searcherToProfile(item))
}

func (s *Server) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
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
	var req legacyProfileWrite
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = existing.Name
	}
	cfg := existing.Config
	if req.EISConfig != nil {
		cfg = *req.EISConfig
	} else if req.Config != nil {
		cfg = *req.Config
	}
	autoAI := existing.AutoAI
	if req.AutoAI != nil {
		autoAI = *req.AutoAI
	}
	out, err := s.Store.UpdateSearcher(r.Context(), u.ID, id, name, cfg, autoAI)
	if err != nil {
		if isUniqueViolation(err) {
			writeError(w, http.StatusConflict, "profile name already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, "update failed")
		return
	}
	writeJSON(w, http.StatusOK, searcherToProfile(out))
}

func (s *Server) handleProfileEISURL(w http.ResponseWriter, r *http.Request) {
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
	writeJSON(w, http.StatusOK, map[string]any{
		"profile_id":  item.ID,
		"searcher_id": item.ID,
		"name":        item.Name,
		"url":         item.Config.ResultsURL(s.Cfg.EISBaseURL),
		"query":       item.Config.QueryValues(),
	})
}
