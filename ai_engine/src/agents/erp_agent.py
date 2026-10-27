from typing import Annotated
from typing_extensions import TypedDict
from langchain_core.messages import BaseMessage, HumanMessage, AIMessage
from langchain.agents import create_agent
from langchain_openai import ChatOpenAI
from src.tools.backend_client import get_customers_tool, get_products_tool, get_sale_orders_tool
from src.core.config import settings
from src.core.logger import setup_logger
import os

logger = setup_logger("erp_agent")

def get_erp_agent():
    """Factory to build and return the ERP Agent based on environment configuration."""
    api_key = settings.LLM_API_KEY
    if not api_key:
        logger.warning("No LLM_API_KEY provided in settings. Using placeholder for testing.")
        api_key = "sk-dummy"
        
    logger.info(f"Initializing LLM provider: {settings.LLM_PROVIDER} with model: {settings.LLM_MODEL}")
    
    # Connect to Groq using ChatOpenAI wrapper if base url is groq
    llm = ChatOpenAI(
        model=settings.LLM_MODEL,
        temperature=0,
        api_key=api_key,
        base_url=settings.LLM_BASE_URL
    )

    tools = [get_customers_tool, get_products_tool, get_sale_orders_tool]
    
    # NOTE: Pyrefly lints might flag create_react_agent as deprecated for langchain,
    # but langgraph.prebuilt.create_react_agent is current for langgraph workflow.
    return create_agent(llm, tools)
