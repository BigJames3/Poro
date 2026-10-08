"""Entry point: python -m moderation."""

import os
import sys

import uvicorn

from moderation.app import create_app
from moderation.config import ConfigError, load_config
from moderation.logs import configure


def main() -> None:
    try:
        config = load_config(os.environ)
    except ConfigError as err:
        sys.stderr.write(f"moderation service: {err}\n")
        sys.exit(1)
    configure(config.log_level)
    uvicorn.run(
        create_app(config),
        host="0.0.0.0",  # noqa: S104 -- the container port is published on 127.0.0.1 by compose.
        port=config.port,
        log_config=None,
        proxy_headers=False,
        server_header=False,
    )


if __name__ == "__main__":
    main()
