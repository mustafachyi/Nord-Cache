package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"nord-api/internal/cache"
)

func TestCatalogNegotiatesBrotli(t *testing.T) {
	store := cache.New()
	store.Set(&cache.Data{
		RawPayload:    []byte("raw"),
		BrotliPayload: []byte("brotli"),
		ETag:          `W/"sha256-test"`,
	})

	request := httptest.NewRequest(http.MethodGet, "/api/servers", nil)
	request.Header.Set("Accept-Encoding", "gzip, br")
	response := httptest.NewRecorder()

	NewMux(store).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if response.Header().Get("Content-Encoding") != "br" {
		t.Fatalf("Content-Encoding = %q, want br", response.Header().Get("Content-Encoding"))
	}
	if response.Header().Get("Vary") != "Accept-Encoding" {
		t.Fatalf("Vary = %q, want Accept-Encoding", response.Header().Get("Vary"))
	}
	if response.Body.String() != "brotli" {
		t.Fatalf("body = %q, want brotli", response.Body.String())
	}
}

func TestCatalogRejectsDisabledBrotli(t *testing.T) {
	store := cache.New()
	store.Set(&cache.Data{
		RawPayload:    []byte("raw"),
		BrotliPayload: []byte("brotli"),
		ETag:          `W/"sha256-test"`,
	})

	request := httptest.NewRequest(http.MethodGet, "/api/servers", nil)
	request.Header.Set("Accept-Encoding", "br;q=0, *;q=1")
	response := httptest.NewRecorder()

	NewMux(store).ServeHTTP(response, request)

	if response.Header().Get("Content-Encoding") != "" {
		t.Fatalf("Content-Encoding = %q, want empty", response.Header().Get("Content-Encoding"))
	}
	if response.Body.String() != "raw" {
		t.Fatalf("body = %q, want raw", response.Body.String())
	}
}

func TestCatalogMatchesWeakETagList(t *testing.T) {
	store := cache.New()
	store.Set(&cache.Data{
		RawPayload:    []byte("raw"),
		BrotliPayload: []byte("brotli"),
		ETag:          `W/"sha256-test"`,
	})

	request := httptest.NewRequest(http.MethodGet, "/api/servers", nil)
	request.Header.Set("If-None-Match", `"other", "sha256-test"`)
	response := httptest.NewRecorder()

	NewMux(store).ServeHTTP(response, request)

	if response.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotModified)
	}
	if response.Body.Len() != 0 {
		t.Fatalf("body length = %d, want 0", response.Body.Len())
	}
}

func TestCatalogHeadReturnsHeadersWithoutBody(t *testing.T) {
	store := cache.New()
	store.Set(&cache.Data{
		RawPayload:    []byte("raw"),
		BrotliPayload: []byte("brotli"),
		ETag:          `W/"sha256-test"`,
	})

	request := httptest.NewRequest(http.MethodHead, "/api/servers", nil)
	response := httptest.NewRecorder()

	NewMux(store).ServeHTTP(response, request)

	result := response.Result()
	defer result.Body.Close()
	body, err := io.ReadAll(result.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if len(body) != 0 {
		t.Fatalf("body length = %d, want 0", len(body))
	}
	if result.ContentLength != 3 {
		t.Fatalf("Content-Length = %d, want 3", result.ContentLength)
	}
}

func TestCatalogReturnsJSONWhileInitializing(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/servers", nil)
	response := httptest.NewRecorder()

	NewMux(cache.New()).ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
	if response.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", response.Header().Get("Content-Type"))
	}
	if response.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatalf("Access-Control-Allow-Origin = %q", response.Header().Get("Access-Control-Allow-Origin"))
	}
	if response.Body.String() != "{\"error\":\"Initializing\"}\n" {
		t.Fatalf("body = %q", response.Body.String())
	}
}
