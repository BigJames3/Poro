# syntax=docker/dockerfile:1
# Build context: repository root (needs packages/shared-go).

FROM golang:1.26-alpine AS builder

WORKDIR /src
COPY packages/shared-go /src/packages/shared-go
COPY services/video/go.mod services/video/go.sum /src/services/video/
WORKDIR /src/services/video
RUN go mod download

COPY services/video /src/services/video
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/video ./cmd/server

FROM alpine:3.22

RUN apk add --no-cache ca-certificates wget && adduser -D -u 10001 app

WORKDIR /app
COPY --from=builder /out/video /app/video
COPY services/video/migrations /app/migrations

ENV MIGRATIONS_PATH=/app/migrations

USER app

EXPOSE 8083

HEALTHCHECK --interval=30s --timeout=3s --start-period=15s --retries=3 \
  CMD wget -qO- http://localhost:8083/health/ready || exit 1

ENTRYPOINT ["/app/video"]
