import asyncio

import httpx
from fastapi import FastAPI
from contextlib import asynccontextmanager
from src.api.routes import router as api_router
from src.api.admin import router as admin_router
from src.core.config import settings
from src.core.runtime_llm import apply_config
from src.graph.workflow import compiled_ai_engine
from src.core.logger import setup_logger

logger = setup_logger("main")


async def _load_llm_config_from_gateway() -> None:
    """Pull the UI-managed LLM config from the Go gateway at boot.

    The gateway is the source of truth and encrypts the key at rest. If it is
    not up yet, keep the environment fallback and retry briefly.
    """
    base = settings.BACKEND_API_URL.rstrip("/")
    url = f"{base}/api/v1/internal/llm-config"
    headers = {}
    if settings.INTERNAL_API_TOKEN:
        headers["X-Internal-Token"] = settings.INTERNAL_API_TOKEN

    for attempt in range(8):
        try:
            async with httpx.AsyncClient(timeout=5.0) as client:
                response = await client.get(url, headers=headers)
            if response.status_code == 200:
                data = response.json()
                apply_config(
                    provider=data.get("provider"),
                    base_url=data.get("base_url"),
                    model=data.get("model"),
                    api_key=data.get("api_key"),
                    max_tokens=data.get("max_tokens"),
                )
                logger.info("Loaded LLM runtime config from gateway")
                return
            logger.warning("Gateway LLM config returned HTTP %s", response.status_code)
        except Exception as exc:  # noqa: BLE001 - startup must not crash
            logger.warning("Could not load LLM config (attempt %d/8): %s", attempt + 1, exc)
        await asyncio.sleep(2)

    logger.warning("Using environment LLM config; gateway config was unavailable")


@asynccontextmanager
async def lifespan(app: FastAPI):
    logger.info("AI Engine booting up...")
    app.state.agent = compiled_ai_engine
    await _load_llm_config_from_gateway()
    logger.info("Agent initialized and loaded into app state.")
    yield
    logger.info("Shutting down AI Engine...")


app = FastAPI(title="AI ERP Operating Layer", version="1.0.0", lifespan=lifespan)

app.include_router(api_router)
app.include_router(admin_router)


@app.get("/health")
async def health_check():
    return {"status": "healthy", "service": "ai_engine"}
