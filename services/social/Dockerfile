# syntax=docker/dockerfile:1
# Build context: repository root (needs packages/shared-go).

FROM golang:1.26-alpine AS builder

WORKDIR /src
COPY packages/shared-go /src/packages/shared-go
COPY services/social/go.mod services/social/go.sum /src/services/social/
WORKDIR /src/services/social
RUN go mod download

COPY services/social /src/services/social
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/social ./cmd/server

FROM alpine:3.22

RUN apk add --no-cache ca-certificates wget && adduser -D -u 10001 app

WORKDIR /app
COPY --from=builder /out/social /app/social
COPY services/social/migrations /app/migrations

ENV MIGRATIONS_PATH=/app/migrations

USER app

EXPOSE 8085

HEALTHCHECK --interval=15s --timeout=3s --start-period=15s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8085/health/ready || exit 1

ENTRYPOINT ["/app/social"]
