"""Mutable LLM configuration for a running AI engine.

The Go gateway owns the persisted settings (encrypted in Postgres) and pushes
changes here. The graph reads this runtime on every model call, so an operator
can rotate keys or switch models from the UI without restarting the container.
"""

from dataclasses import dataclass, replace
from threading import RLock
from typing import Optional

from src.core.config import settings


@dataclass(frozen=True)
class LLMConfig:
    provider: str
    api_key: str
    base_url: str
    model: str
    max_tokens: int

    def redacted(self) -> dict:
        return {
            "provider": self.provider,
            "base_url": self.base_url,
            "model": self.model,
            "max_tokens": self.max_tokens,
            "has_api_key": bool(self.api_key),
        }


_lock = RLock()
_config = LLMConfig(
    provider=settings.LLM_PROVIDER,
    api_key=settings.LLM_API_KEY or "",
    base_url=settings.LLM_BASE_URL,
    model=settings.LLM_MODEL,
    max_tokens=settings.LLM_MAX_TOKENS,
)


def snapshot() -> LLMConfig:
    with _lock:
        return _config


def apply_config(
    *,
    provider: Optional[str] = None,
    api_key: Optional[str] = None,
    base_url: Optional[str] = None,
    model: Optional[str] = None,
    max_tokens: Optional[int] = None,
) -> LLMConfig:
    """Apply a partial update. Empty api_key means keep the current key."""
    global _config
    with _lock:
        updates = {}
        if provider:
            updates["provider"] = provider.strip()
        if base_url:
            updates["base_url"] = base_url.strip()
        if model:
            updates["model"] = model.strip()
        if max_tokens and max_tokens > 0:
            updates["max_tokens"] = min(int(max_tokens), 8192)
        if api_key:
            updates["api_key"] = api_key.strip()
        _config = replace(_config, **updates)
        return _config


def redacted() -> dict:
    return snapshot().redacted()
