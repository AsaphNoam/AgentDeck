package server

import (
	"log/slog"
	"net/http"
	"time"
)

// devOrigin is the local Vite dev server origin allowed by CORS so the UI can be
// developed against `vite dev` (:5173) while hitting the Go API.
const devOrigin = "http://localhost:5173"

// statusRecorder captures the response status code for request logging. It
// defaults to 200 since WriteHeader is not always called explicitly.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// cors adds permissive CORS headers for the local Vite origin and short-circuits
// OPTIONS preflight requests with 204.
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", devOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Vary", "Origin")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// requestLog logs every request via slog with method, path, status and duration.
func requestLog(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"dur_ms", time.Since(start).Milliseconds(),
		)
	})
}

// withMiddleware wraps h with CORS then request logging (outermost logs first).
func withMiddleware(log *slog.Logger, h http.Handler) http.Handler {
	return requestLog(log, cors(h))
}
