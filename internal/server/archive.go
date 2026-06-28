package server

import (
	"net/http"
	"strconv"

	persistarchive "github.com/agentdeck/agentdeck/internal/archive"
	"github.com/agentdeck/agentdeck/internal/runtime"
)

func (s *Server) handleArchive(w http.ResponseWriter, r *http.Request) {
	limit, err := parseIntQuery(r, "limit", 50)
	if err != nil || limit < 1 || limit > 200 {
		writeAPIError(w, apiError(runtime.CodeValidation, "limit must be between 1 and 200"))
		return
	}
	offset, err := parseIntQuery(r, "offset", 0)
	if err != nil || offset < 0 {
		writeAPIError(w, apiError(runtime.CodeValidation, "offset must be >= 0"))
		return
	}
	var active *bool
	switch r.URL.Query().Get("active") {
	case "":
	case "true":
		v := true
		active = &v
	case "false":
		v := false
		active = &v
	default:
		writeAPIError(w, apiError(runtime.CodeValidation, "active must be true or false"))
		return
	}
	resp, err := persistarchive.New(s.stateStore.DB()).Search(persistarchive.Query{
		Q:      r.URL.Query().Get("q"),
		Limit:  limit,
		Offset: offset,
		Active: active,
	})
	if err != nil {
		writeAPIError(w, apiError(runtime.CodeInternal, err.Error()))
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func parseIntQuery(r *http.Request, key string, def int) (int, error) {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return def, nil
	}
	return strconv.Atoi(raw)
}
