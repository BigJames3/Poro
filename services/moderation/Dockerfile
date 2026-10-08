# syntax=docker/dockerfile:1
# Build context: services/moderation. Targets: runtime (default) and migrate.

FROM python:3.13-slim AS build
RUN pip install --no-cache-dir uv==0.11.32
WORKDIR /app
ENV UV_COMPILE_BYTECODE=1 UV_LINK_MODE=copy UV_PROJECT_ENVIRONMENT=/app/.venv UV_PYTHON_DOWNLOADS=never
COPY pyproject.toml uv.lock ./
RUN uv sync --frozen --no-dev --no-install-project
COPY src ./src
RUN uv sync --frozen --no-dev --no-editable

FROM python:3.13-slim AS base
RUN useradd --uid 10001 --no-create-home --shell /usr/sbin/nologin moderation
WORKDIR /app
COPY --from=build /app/.venv /app/.venv
ENV PATH=/app/.venv/bin:$PATH PYTHONUNBUFFERED=1 PYTHONDONTWRITEBYTECODE=1

FROM base AS migrate
COPY alembic.ini ./
COPY migrations ./migrations
USER moderation
ENTRYPOINT ["alembic", "upgrade", "head"]

FROM base AS runtime
USER moderation
EXPOSE 8087
HEALTHCHECK --interval=15s --timeout=3s --start-period=15s --retries=3 \
  CMD ["python", "-c", "import urllib.request; urllib.request.urlopen('http://127.0.0.1:8087/health/ready', timeout=2)"]
CMD ["python", "-m", "moderation"]
