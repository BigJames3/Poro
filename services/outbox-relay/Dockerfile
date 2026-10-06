# Build context: repository root (needs packages/shared-go).
FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY packages/shared-go /src/packages/shared-go
COPY services/outbox-relay/go.mod services/outbox-relay/go.sum /src/services/outbox-relay/
WORKDIR /src/services/outbox-relay
RUN go mod download
COPY services/outbox-relay /src/services/outbox-relay
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/relay ./cmd/relay

FROM alpine:3.22
RUN apk add --no-cache ca-certificates wget && adduser -D -H -u 10001 app
COPY --from=builder /out/relay /app/relay
USER app
EXPOSE 9100
HEALTHCHECK --interval=15s --timeout=3s --start-period=10s --retries=3 \
  CMD wget -qO- http://127.0.0.1:9100/health/ready >/dev/null || exit 1
ENTRYPOINT ["/app/relay"]
