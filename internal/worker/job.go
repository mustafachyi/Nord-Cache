package worker

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"nord-api/internal/cache"
	"nord-api/internal/nord"
)

func Start(ctx context.Context, store *cache.Store, interval time.Duration) {
	execute(store)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			execute(store)
		}
	}
}

func execute(store *cache.Store) {
	payload, err := nord.FetchAndProcess()
	if err != nil {
		log.Printf("sync failed: %v", err)
		return
	}

	gzipPayload, err := compress(payload)
	if err != nil {
		log.Printf("compression failed: %v", err)
		return
	}

	eTag := fmt.Sprintf(`"%s"`, strconv.FormatInt(time.Now().UnixNano(), 16))

	store.Set(&cache.Data{
		RawPayload:  payload,
		GzipPayload: gzipPayload,
		ETag:        eTag,
	})
}

func compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err != nil {
		return nil, err
	}

	if _, err := writer.Write(data); err != nil {
		return nil, err
	}

	if err := writer.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
