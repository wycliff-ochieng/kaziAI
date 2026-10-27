from langchain_openai import ChatOpenAI
from src.core.config import settings
from src.core.logger import setup_logger

logger = setup_logger("llm_factory")


def get_llm(temperature: float = 0.0):
    """Return a chat model capped to the deployment's token budget.

    The output cap matters: Groq's on-demand tier rejects requests whose
    expected completion exceeds the per-minute output budget, so we never let
    the SDK fall back to its 2048-token default.
    """
    api_key = settings.LLM_API_KEY or "sk-dummy"

    return ChatOpenAI(
        model=settings.LLM_MODEL,
        temperature=temperature,
        api_key=api_key,
        base_url=settings.LLM_BASE_URL,
        max_tokens=settings.LLM_MAX_TOKENS,
        timeout=settings.LLM_REQUEST_TIMEOUT,
        max_retries=1,
    )
