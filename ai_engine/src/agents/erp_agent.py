from typing import Annotated
from typing_extensions import TypedDict
from langchain_core.messages import BaseMessage, HumanMessage, AIMessage
from langchain.agents import create_agent
from src.tools.backend_client import get_customers_tool, get_products_tool, get_sale_orders_tool
from src.core.llm import get_llm
from src.core.logger import setup_logger
import os

logger = setup_logger("erp_agent")

def get_erp_agent():
    """Factory to build and return the ERP Agent with the current runtime LLM."""
    tools = [get_customers_tool, get_products_tool, get_sale_orders_tool]
    
    # NOTE: Pyrefly lints might flag create_react_agent as deprecated for langchain,
    # but langgraph.prebuilt.create_react_agent is current for langgraph workflow.
    return create_agent(get_llm(), tools)
