from fastapi import Request
from src.agents.erp_agent import get_erp_agent

async def get_agent(request: Request):
    """Dependency to inject the ERP Agent instance."""
    # Assuming app.state.agent is initialized during lifespan
    return request.app.state.agent
