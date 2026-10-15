from pathlib import Path

import pytest

from moderation.config import ConfigError, load_config

PROD = {
    "APP_ENV": "prod",
    "DATABASE_URL": "postgresql://poro:secret@db:5432/poro_moderation?sslmode=verify-full&application_name=m",
    "REDIS_URL": "rediss://redis:6379/6",
    "REVOCATION_REDIS_URL": "rediss://revocations:6380/0",
    "KAFKA_BROKERS": "kafka-1:9092, kafka-2:9092",
}


def test_dev_defaults() -> None:
    config = load_config(
        {"APP_ENV": "dev", "DATABASE_URL": "postgresql://poro:x@localhost:5433/poro_moderation"}
    )
    assert config.env == "dev"
    assert config.port == 8087
    assert config.log_level == "DEBUG"
    assert config.database_url == "postgresql+asyncpg://poro:x@localhost:5433/poro_moderation"
    assert config.database_ssl == "prefer"
    assert config.redis_url is None
    assert config.revocation_redis_url is None, "optional for a service started alone in dev"
    assert config.kafka_brokers == ["localhost:9092"]
    assert config.kafka_consumer_enabled is True
    assert config.report_threshold == 3


def test_hardened_prod() -> None:
    config = load_config({**PROD, "REPORT_THRESHOLD": "5", "KAFKA_CONSUMER_ENABLED": "false"})
    assert config.database_ssl == "verify-full"
    assert config.database_url.endswith("/poro_moderation?application_name=m")
    assert config.kafka_brokers == ["kafka-1:9092", "kafka-2:9092"]
    assert config.log_level == "INFO"
    assert config.report_threshold == 5
    assert config.kafka_consumer_enabled is False
    assert config.revocation_redis_url == "rediss://revocations:6380/0"


@pytest.mark.parametrize(
    ("override", "message"),
    [
        ({"APP_ENV": "production"}, "APP_ENV"),
        ({"APP_ENV": ""}, "APP_ENV is required"),
        ({"APP_ENV": "  "}, "APP_ENV is required"),
        ({"PORT": "70000"}, "PORT"),
        ({"LOG_LEVEL": "verbose"}, "LOG_LEVEL"),
        ({"DATABASE_URL": "mysql://x/y"}, "DATABASE_URL must be"),
        ({"DATABASE_URL": "postgresql://poro:x@db/poro_moderation?sslmode=disable"}, "outside dev"),
        ({"DATABASE_URL": "postgresql://poro:x@db/poro_moderation?sslmode=sometimes"}, "sslmode must be"),
        ({"REDIS_URL": ""}, "REDIS_URL is required"),
        ({"REDIS_URL": "http://redis"}, "REDIS_URL must be"),
        ({"REVOCATION_REDIS_URL": ""}, "REVOCATION_REDIS_URL is required outside dev"),
        ({"REVOCATION_REDIS_URL": "http://redis"}, "REVOCATION_REDIS_URL must be"),
        ({"JWKS_URL": "ftp://auth"}, "JWKS_URL"),
        ({"KAFKA_BROKERS": " , "}, "KAFKA_BROKERS"),
        ({"REPORT_THRESHOLD": "0"}, "REPORT_THRESHOLD"),
        ({"RULES_PATH": "/nope.yml"}, "RULES_PATH"),
    ],
)
def test_rejects(override: dict[str, str], message: str) -> None:
    with pytest.raises(ConfigError, match=message):
        load_config({**PROD, **override})


def test_custom_rules_path(tmp_path: Path) -> None:
    rules = tmp_path / "rules.yml"
    rules.write_text("block: {}\n", encoding="utf-8")
    assert load_config({**PROD, "RULES_PATH": str(rules)}).rules_path == rules


def test_app_env_has_no_default() -> None:
    """A forgotten APP_ENV stops the service instead of running it in dev."""
    with pytest.raises(ConfigError, match="APP_ENV is required"):
        load_config({"DATABASE_URL": "postgresql://poro:x@localhost:5433/poro_moderation"})
