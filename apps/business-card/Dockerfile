# syntax=docker/dockerfile:1
# Build context: apps/business-card. PUBLIC_URL is the card's final https://
# address (canonical link, link previews, QR code).

FROM node:24-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --omit=dev --no-audit --no-fund
COPY profile.json ./
COPY assets ./assets
COPY src ./src
COPY scripts/build.mjs ./scripts/
ARG PUBLIC_URL
RUN test -n "$PUBLIC_URL" || (echo "--build-arg PUBLIC_URL=https://… est requis" && exit 1)
RUN PUBLIC_URL="$PUBLIC_URL" node scripts/build.mjs --prod

FROM nginxinc/nginx-unprivileged:1.29-alpine AS runtime
COPY deploy/nginx.conf /etc/nginx/conf.d/default.conf
COPY deploy/security-headers.conf /etc/nginx/snippets/security-headers.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/healthz >/dev/null || exit 1
