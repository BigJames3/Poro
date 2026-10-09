# syntax=docker/dockerfile:1
# Build context: services/order. Targets: runtime (default) and migrate.

FROM node:24-alpine AS build
WORKDIR /app
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY tsconfig.json tsconfig.build.json nest-cli.json prisma.config.ts ./
COPY prisma ./prisma
COPY src ./src
# prisma generate never connects; the config just refuses an unset URL.
RUN DATABASE_URL=postgresql://build@localhost:5432/build npm run build

FROM build AS prod-deps
COPY scripts/prune-runtime-deps.mjs ./scripts/
RUN npm prune --omit=dev --no-audit --no-fund && node scripts/prune-runtime-deps.mjs

FROM node:24-alpine AS migrate
RUN apk add --no-cache openssl
WORKDIR /app
COPY --from=build /app/node_modules ./node_modules
COPY package.json prisma.config.ts ./
COPY prisma ./prisma
USER node
ENTRYPOINT ["node", "node_modules/prisma/build/index.js", "migrate", "deploy"]

FROM node:24-alpine AS runtime
RUN apk add --no-cache wget
ENV NODE_ENV=production
WORKDIR /app
COPY --from=prod-deps /app/node_modules ./node_modules
COPY --from=build /app/dist ./dist
COPY package.json ./
USER node
EXPOSE 8091
HEALTHCHECK --interval=15s --timeout=3s --start-period=15s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8091/health/ready >/dev/null || exit 1
CMD ["node", "--enable-source-maps", "dist/main.js"]
