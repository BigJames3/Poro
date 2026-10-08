"""Environment configuration. Staging and prod refuse the shortcuts that are
only safe on a laptop: plaintext Postgres and in-memory rate limits."""

from collections.abc import Mapping
from dataclasses import dataclass
from pathlib import Path
from urllib.parse import parse_qsl, urlencode, urlsplit, urlunsplit

APP_ENVS = ("dev", "staging", "prod")
LOG_LEVELS = ("DEBUG", "INFO", "WARNING", "ERROR")
SSL_MODES = ("disable", "allow", "prefer", "require", "verify-ca", "verify-full")
DEFAULT_RULES = Path(__file__).parent / "rules" / "fr.yml"


class ConfigError(ValueError):
    """The environment is unusable; the message lists every problem."""


@dataclass(frozen=True)
class Config:
    env: str
    port: int
    log_level: str
    database_url: str
    """SQLAlchemy URL for asyncpg, without sslmode."""
    database_ssl: str
    redis_url: str | None
    jwks_url: str
    kafka_brokers: list[str]
    kafka_consumer_enabled: bool
    report_threshold: int
    rules_path: Path


def _database(raw: str, dev: bool, errors: list[str]) -> tuple[str, str]:
    parts = urlsplit(raw)
    if parts.scheme not in ("postgres", "postgresql") or not parts.hostname:
        errors.append("DATABASE_URL must be a postgresql:// URL")
        return "", "disable"
    query = dict(parse_qsl(parts.query))
    ssl = query.pop("sslmode", "prefer")
    if ssl not in SSL_MODES:
        errors.append(f"DATABASE_URL sslmode must be one of {', '.join(SSL_MODES)}")
    elif not dev and ssl not in ("require", "verify-ca", "verify-full"):
        errors.append("DATABASE_URL must set sslmode=require, verify-ca or verify-full outside dev")
    url = urlunsplit(("postgresql+asyncpg", parts.netloc, parts.path, urlencode(query), ""))
    return url, ssl


def _int(env: Mapping[str, str], name: str, default: str, low: int, high: int, errors: list[str]) -> int:
    raw = env.get(name, default)
    if not raw.isdigit() or not low <= int(raw) <= high:
        errors.append(f"{name} must be an integer between {low} and {high}")
        return int(default)
    return int(raw)


def load_config(env: Mapping[str, str]) -> Config:
    errors: list[str] = []
    app_env = env.get("APP_ENV", "dev")
    if app_env not in APP_ENVS:
        errors.append(f'APP_ENV must be dev, staging or prod, got "{app_env}"')
    dev = app_env == "dev"

    port = _int(env, "PORT", "8087", 1, 65535, errors)
    log_level = env.get("LOG_LEVEL", "DEBUG" if dev else "INFO").upper()
    if log_level not in LOG_LEVELS:
        errors.append(f"LOG_LEVEL must be one of {', '.join(LOG_LEVELS)}")

    database_url, database_ssl = _database(env.get("DATABASE_URL", ""), dev, errors)

    redis_url = env.get("REDIS_URL", "").strip() or None
    if redis_url is not None and urlsplit(redis_url).scheme not in ("redis", "rediss"):
        errors.append("REDIS_URL must be a redis:// or rediss:// URL")
    if not dev and redis_url is None:
        errors.append("REDIS_URL is required outside dev: rate limits must be shared by replicas")

    jwks_url = env.get("JWKS_URL", "http://localhost:8081/.well-known/jwks.json")
    if urlsplit(jwks_url).scheme not in ("http", "https"):
        errors.append("JWKS_URL must be an http(s) URL")

    brokers = [b.strip() for b in env.get("KAFKA_BROKERS", "localhost:9092").split(",") if b.strip()]
    if not brokers:
        errors.append("KAFKA_BROKERS is required")
    consumer_enabled = env.get("KAFKA_CONSUMER_ENABLED", "true") != "false"

    threshold = _int(env, "REPORT_THRESHOLD", "3", 1, 1000, errors)
    rules_path = Path(env.get("RULES_PATH", str(DEFAULT_RULES)))
    if not rules_path.is_file():
        errors.append(f"RULES_PATH {rules_path} is not a file")

    if errors:
        raise ConfigError("invalid config: " + "; ".join(errors))
    return Config(
        env=app_env,
        port=port,
        log_level=log_level,
        database_url=database_url,
        database_ssl=database_ssl,
        redis_url=redis_url,
        jwks_url=jwks_url,
        kafka_brokers=brokers,
        kafka_consumer_enabled=consumer_enabled,
        report_threshold=threshold,
        rules_path=rules_path,
    )
