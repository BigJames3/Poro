# syntax=docker/dockerfile:1
# Build context: repository root (needs packages/shared-go).

FROM golang:1.26-alpine AS builder

WORKDIR /src
COPY packages/shared-go /src/packages/shared-go
COPY services/search/go.mod services/search/go.sum /src/services/search/
WORKDIR /src/services/search
RUN go mod download

COPY services/search /src/services/search
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/search ./cmd/server

FROM alpine:3.22

RUN apk add --no-cache ca-certificates wget && adduser -D -u 10001 app

WORKDIR /app
COPY --from=builder /out/search /app/search
COPY services/search/migrations /app/migrations

ENV MIGRATIONS_PATH=/app/migrations

USER app

EXPOSE 8088

HEALTHCHECK --interval=15s --timeout=3s --start-period=15s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8088/health/ready || exit 1

ENTRYPOINT ["/app/search"]
