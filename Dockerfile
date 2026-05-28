FROM golang:1.26.3-alpine AS builder

RUN apk add --no-cache ca-certificates tzdata

RUN adduser -D -g '' -H -s /sbin/nologin appuser

WORKDIR /app

COPY go.mod ./
COPY cmd/ ./cmd/
COPY internal/ ./internal/

RUN go mod tidy

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOAMD64=v3 go build -trimpath -ldflags="-s -w" -o /api ./cmd/api

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group

COPY --from=builder /api /api

USER appuser

EXPOSE 8080

ENTRYPOINT ["/api"]