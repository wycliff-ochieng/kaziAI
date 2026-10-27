from langchain_openai import ChatOpenAI
from src.core.config import settings
from src.core.runtime_llm import snapshot
from src.core.logger import setup_logger

logger = setup_logger("llm_factory")


def get_llm(temperature: float = 0.0):
    """Return a chat model using the current runtime configuration.

    The output cap matters: Groq's on-demand tier rejects requests whose
    expected completion exceeds the per-minute output budget, so we never let
    the SDK fall back to its 2048-token default.
    """
    cfg = snapshot()
    api_key = cfg.api_key or "sk-dummy"

    return ChatOpenAI(
        model=cfg.model,
        temperature=temperature,
        api_key=api_key,
        base_url=cfg.base_url,
        max_tokens=cfg.max_tokens,
        timeout=settings.LLM_REQUEST_TIMEOUT,
        max_retries=1,
    )
