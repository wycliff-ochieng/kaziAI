import os
from pydantic_settings import BaseSettings, SettingsConfigDict
from typing import Optional

BASE_DIR = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
ENV_PATH = os.path.join(BASE_DIR, ".env")

class Settings(BaseSettings):
    ENVIRONMENT: str = "development"
    # LLM Configuration
    LLM_PROVIDER: str = "groq"
    LLM_API_KEY: Optional[str] = None
    LLM_BASE_URL: str = "https://api.groq.com/openai/v1"
    LLM_MODEL: str = "llama3-70b-8192"
    # Groq on-demand tiers enforce a low output-tokens-per-minute budget
    # (e.g. 1000 OTPM). LangChain defaults to a 2048-token completion request,
    # which trips a 429 before the model is even reached. Keep this small.
    LLM_MAX_TOKENS: int = 400
    LLM_REQUEST_TIMEOUT: float = 20.0
    # Groq on-demand enforces ~1000 output-tokens/minute, so a single burst of
    # turns trips 429. Retry with the provider's own backoff hint.
    LLM_MAX_RETRIES_ON_429: int = 4
    LLM_429_MAX_BACKOFF_SECONDS: float = 30.0
    # Session memory TTL for multi-turn chaining (seconds).
    SESSION_TTL_SECONDS: int = 1800
    
    # Backend Integrations
    BACKEND_API_URL: str = "http://localhost:8080/api/v1"
    ODOO_URL: str = "http://localhost:8069"
    
    model_config = SettingsConfigDict(env_file=ENV_PATH, env_file_encoding="utf-8", extra="ignore")

settings = Settings()
