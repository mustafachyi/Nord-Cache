package server

import (
	"net/http"
	"strings"

	"nord-api/internal/cache"
)

func NewMux(store *cache.Store) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	mux.HandleFunc("GET /api/servers", func(w http.ResponseWriter, r *http.Request) {
		data := store.Get()
		if data == nil {
			http.Error(w, `{"error":"Initializing"}`, http.StatusServiceUnavailable)
			return
		}

		clientETag := r.Header.Get("If-None-Match")
		if clientETag == data.ETag || clientETag == `W/`+data.ETag {
			w.Header().Set("ETag", data.ETag)
			w.Header().Set("Cache-Control", "public, no-transform, max-age=300")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.WriteHeader(http.StatusNotModified)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("ETag", data.ETag)
		w.Header().Set("Cache-Control", "public, no-transform, max-age=300")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		if strings.Contains(r.Header.Get("Accept-Encoding"), "br") {
			w.Header().Set("Content-Encoding", "br")
			w.WriteHeader(http.StatusOK)
			w.Write(data.BrotliPayload)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write(data.RawPayload)
	})

	return mux
}
