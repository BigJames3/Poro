"""Request bodies. Unknown fields are refused."""

import uuid
from typing import Literal

from pydantic import BaseModel, ConfigDict, Field

Reason = Literal["spam", "nudity", "violence", "harassment", "hate", "fraud", "other"]


class ReportRequest(BaseModel):
    model_config = ConfigDict(extra="forbid")

    target_type: Literal["video", "comment", "user"]
    target_id: uuid.UUID
    reason: Reason
    comment: str | None = Field(default=None, max_length=500)


class DecisionRequest(BaseModel):
    model_config = ConfigDict(extra="forbid")

    action: Literal["remove", "dismiss", "restore"]
    reason: Reason | None = None
    note: str | None = Field(default=None, max_length=500)
