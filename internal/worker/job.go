package worker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"log"
	"time"

	"github.com/andybalholm/brotli"

	"nord-api/internal/cache"
	"nord-api/internal/nord"
)

func Start(ctx context.Context, store *cache.Store, interval time.Duration) {
	execute(ctx, store)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			execute(ctx, store)
		}
	}
}

func execute(ctx context.Context, store *cache.Store) {
	payload, err := nord.FetchAndProcess(ctx)
	if err != nil {
		if ctx.Err() == nil {
			log.Printf("catalog refresh failed: %v", err)
		}
		return
	}

	etag := createETag(payload)
	if current := store.Get(); current != nil && current.ETag == etag {
		return
	}

	brotliPayload, err := compress(payload)
	if err != nil {
		log.Printf("catalog compression failed: %v", err)
		return
	}

	store.Set(&cache.Data{
		RawPayload:    payload,
		BrotliPayload: brotliPayload,
		ETag:          etag,
	})
}

func createETag(data []byte) string {
	digest := sha256.Sum256(data)
	return `W/"sha256-` + base64.RawURLEncoding.EncodeToString(digest[:]) + `"`
}

func compress(data []byte) ([]byte, error) {
	var buffer bytes.Buffer
	writer := brotli.NewWriterLevel(&buffer, brotli.BestCompression)

	if _, err := writer.Write(data); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}
