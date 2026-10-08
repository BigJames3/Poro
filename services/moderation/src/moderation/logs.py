"""JSON logs on stdout, one object per line, like the other services."""

import json
import logging
from datetime import UTC, datetime

from moderation import SERVICE_NAME, SERVICE_VERSION

# color_message is the ANSI copy of msg that uvicorn adds.
_STANDARD = set(logging.makeLogRecord({}).__dict__) | {"message", "asctime", "color_message"}


class JsonFormatter(logging.Formatter):
    def format(self, record: logging.LogRecord) -> str:
        doc: dict[str, object] = {
            "ts": datetime.fromtimestamp(record.created, UTC).isoformat(),
            "level": record.levelname.lower(),
            "logger": record.name,
            "msg": record.getMessage(),
            "service": SERVICE_NAME,
            "version": SERVICE_VERSION,
        }
        doc.update({k: v for k, v in record.__dict__.items() if k not in _STANDARD})
        if record.exc_info:
            doc["error"] = self.formatException(record.exc_info)
        return json.dumps(doc, default=str, ensure_ascii=False)


def configure(level: str) -> None:
    handler = logging.StreamHandler()
    handler.setFormatter(JsonFormatter())
    root = logging.getLogger()
    root.handlers[:] = [handler]
    root.setLevel(level)
    for noisy in ("aiokafka", "uvicorn.access"):
        logging.getLogger(noisy).setLevel(logging.WARNING)
