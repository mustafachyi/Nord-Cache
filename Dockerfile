FROM golang:1.26.5-alpine AS builder

RUN apk add --no-cache ca-certificates
RUN addgroup -S -g 10001 appgroup && adduser -S -D -H -u 10001 -G appgroup -s /sbin/nologin appuser

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 go build -mod=readonly -trimpath -ldflags="-s -w" -o /api ./cmd/api

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /etc/group /etc/group
COPY --from=builder /api /api

USER appuser

EXPOSE 8080

ENTRYPOINT ["/api"]