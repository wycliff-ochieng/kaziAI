"""Internal admin API used by the Go gateway.

These endpoints are never exposed through the public reverse proxy. The
gateway calls them with X-Internal-Token to hot-apply LLM settings.
"""

import hmac

import httpx
from fastapi import APIRouter, Header, HTTPException
from pydantic import BaseModel

from src.core.config import settings
from src.core.runtime_llm import apply_config, redacted, snapshot

router = APIRouter(prefix="/api/v1/admin", tags=["admin"])


class LLMConfigBody(BaseModel):
    provider: str | None = None
    base_url: str | None = None
    model: str | None = None
    api_key: str | None = None
    max_tokens: int | None = None


def _require_internal_token(token: str | None) -> None:
    expected = settings.INTERNAL_API_TOKEN
    if not expected:
        # Local development only. Production must set INTERNAL_API_TOKEN.
        return
    if not token or not hmac.compare_digest(token, expected):
        raise HTTPException(status_code=401, detail="invalid internal token")


@router.get("/llm")
async def get_llm_config(x_internal_token: str | None = Header(default=None)):
    _require_internal_token(x_internal_token)
    return {"status": "success", "data": redacted()}


@router.post("/llm")
async def update_llm_config(
    body: LLMConfigBody,
    x_internal_token: str | None = Header(default=None),
):
    _require_internal_token(x_internal_token)
    cfg = apply_config(
        provider=body.provider,
        base_url=body.base_url,
        model=body.model,
        api_key=body.api_key,
        max_tokens=body.max_tokens,
    )
    return {"status": "success", "data": cfg.redacted()}


@router.post("/llm/test")
async def test_llm_config(
    body: LLMConfigBody,
    x_internal_token: str | None = Header(default=None),
):
    """Probe the provider's /models endpoint without mutating runtime config."""
    _require_internal_token(x_internal_token)
    current = snapshot()
    base_url = (body.base_url or current.base_url).rstrip("/")
    api_key = body.api_key or current.api_key
    model = body.model or current.model

    if not api_key:
        return {"ok": False, "message": "No API key configured"}

    try:
        async with httpx.AsyncClient(timeout=15.0) as client:
            response = await client.get(
                f"{base_url}/models",
                headers={"Authorization": f"Bearer {api_key}"},
            )
        if response.status_code >= 400:
            detail = response.text[:300]
            return {
                "ok": False,
                "message": f"Provider returned HTTP {response.status_code}: {detail}",
                "model": model,
            }
        return {
            "ok": True,
            "message": f"Provider reachable; model configured as {model}",
            "model": model,
        }
    except Exception as exc:  # noqa: BLE001 - surface the provider error to the UI
        return {"ok": False, "message": f"Provider test failed: {exc}", "model": model}
