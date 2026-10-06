# syntax=docker/dockerfile:1
# Build context: repository root (needs packages/shared-go).

FROM golang:1.26-alpine AS builder

WORKDIR /src
COPY packages/shared-go /src/packages/shared-go
COPY services/auth/go.mod services/auth/go.sum /src/services/auth/
WORKDIR /src/services/auth
RUN go mod download

COPY services/auth /src/services/auth
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/auth ./cmd/server

FROM alpine:3.22

RUN apk add --no-cache ca-certificates wget && adduser -D -u 10001 app

WORKDIR /app
COPY --from=builder /out/auth /app/auth
COPY services/auth/migrations /app/migrations

ENV MIGRATIONS_PATH=/app/migrations

USER app

EXPOSE 8081

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
  CMD wget -qO- http://localhost:8081/health/ready || exit 1

ENTRYPOINT ["/app/auth"]
