package server

import (
	"io"
	"net/http"
	"strconv"
	"strings"

	"nord-api/internal/cache"
)

const catalogCacheControl = "public, max-age=300"

func NewMux(store *cache.Store) *http.ServeMux {
	mux := http.NewServeMux()

	healthHandler := func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Cache-Control", "no-store")
		response.Header().Set("Content-Type", "text/plain; charset=utf-8")
		response.Header().Set("X-Content-Type-Options", "nosniff")
		if request.Method == http.MethodHead {
			response.Header().Set("Content-Length", "2")
			response.WriteHeader(http.StatusOK)
			return
		}
		_, _ = io.WriteString(response, "OK")
	}

	catalogHandler := func(response http.ResponseWriter, request *http.Request) {
		setCatalogCORSHeaders(response.Header())

		if request.Method == http.MethodOptions {
			response.Header().Set("Allow", "GET, HEAD, OPTIONS")
			response.WriteHeader(http.StatusNoContent)
			return
		}

		data := store.Get()
		if data == nil {
			writeJSONError(response, http.StatusServiceUnavailable, "Initializing")
			return
		}

		setCatalogHeaders(response.Header(), data.ETag)
		if matchesIfNoneMatch(request.Header.Get("If-None-Match"), data.ETag) {
			response.WriteHeader(http.StatusNotModified)
			return
		}

		payload := data.RawPayload
		if acceptsEncoding(request.Header.Get("Accept-Encoding"), "br") {
			payload = data.BrotliPayload
			response.Header().Set("Content-Encoding", "br")
		}

		response.Header().Set("Content-Length", strconv.Itoa(len(payload)))
		if request.Method == http.MethodHead {
			response.WriteHeader(http.StatusOK)
			return
		}

		response.WriteHeader(http.StatusOK)
		_, _ = response.Write(payload)
	}

	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("HEAD /health", healthHandler)
	mux.HandleFunc("GET /api/servers", catalogHandler)
	mux.HandleFunc("HEAD /api/servers", catalogHandler)
	mux.HandleFunc("OPTIONS /api/servers", catalogHandler)

	return mux
}

func setCatalogHeaders(headers http.Header, etag string) {
	headers.Set("Access-Control-Expose-Headers", "ETag")
	headers.Set("Cache-Control", catalogCacheControl)
	headers.Set("Content-Type", "application/json; charset=utf-8")
	headers.Set("ETag", etag)
	headers.Set("Vary", "Accept-Encoding")
	headers.Set("X-Content-Type-Options", "nosniff")
}

func setCatalogCORSHeaders(headers http.Header) {
	headers.Set("Access-Control-Allow-Headers", "If-None-Match")
	headers.Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
	headers.Set("Access-Control-Allow-Origin", "*")
	headers.Set("Access-Control-Max-Age", "86400")
}

func writeJSONError(response http.ResponseWriter, status int, message string) {
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.Header().Set("X-Content-Type-Options", "nosniff")
	response.WriteHeader(status)
	_, _ = io.WriteString(response, `{"error":`+strconv.Quote(message)+"}\n")
}

func matchesIfNoneMatch(value string, etag string) bool {
	if value == "" {
		return false
	}

	target := removeWeakPrefix(etag)
	for _, candidate := range strings.Split(value, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || removeWeakPrefix(candidate) == target {
			return true
		}
	}

	return false
}

func removeWeakPrefix(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && strings.EqualFold(value[:2], "W/") {
		return strings.TrimSpace(value[2:])
	}
	return value
}

func acceptsEncoding(value string, encoding string) bool {
	explicitFound := false
	explicitQuality := 0.0
	wildcardFound := false
	wildcardQuality := 0.0

	for _, item := range strings.Split(value, ",") {
		parts := strings.Split(item, ";")
		name := strings.TrimSpace(parts[0])
		quality := 1.0

		for _, parameter := range parts[1:] {
			key, parameterValue, found := strings.Cut(strings.TrimSpace(parameter), "=")
			if !found || !strings.EqualFold(strings.TrimSpace(key), "q") {
				continue
			}

			parsedQuality, err := strconv.ParseFloat(strings.TrimSpace(parameterValue), 64)
			if err != nil || parsedQuality < 0 || parsedQuality > 1 {
				quality = 0
			} else {
				quality = parsedQuality
			}
		}

		if strings.EqualFold(name, encoding) {
			explicitFound = true
			if quality > explicitQuality {
				explicitQuality = quality
			}
		} else if name == "*" {
			wildcardFound = true
			if quality > wildcardQuality {
				wildcardQuality = quality
			}
		}
	}

	if explicitFound {
		return explicitQuality > 0
	}
	return wildcardFound && wildcardQuality > 0
}
