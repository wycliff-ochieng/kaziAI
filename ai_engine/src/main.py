from fastapi import FastAPI
from contextlib import asynccontextmanager
from src.api.routes import router as api_router
from src.graph.workflow import compiled_ai_engine
from src.core.logger import setup_logger

logger = setup_logger("main")

@asynccontextmanager
async def lifespan(app: FastAPI):
    logger.info("AI Engine booting up...")
    app.state.agent = compiled_ai_engine
    logger.info("Agent initialized and loaded into app state.")
    yield
    logger.info("Shutting down AI Engine...")

app = FastAPI(title="AI ERP Operating Layer", version="1.0.0", lifespan=lifespan)

app.include_router(api_router)

@app.get("/health")
async def health_check():
    return {"status": "healthy", "service": "ai_engine"}
