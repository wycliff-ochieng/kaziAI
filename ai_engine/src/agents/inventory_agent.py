from langgraph.prebuilt import create_react_agent
from src.tools.backend_client import get_products_tool
from src.core.llm import get_llm
from src.graph.state import AgentState

inventory_tools = [get_products_tool] # Additional mock tools: check_stock, transfer_inventory

inventory_bot = create_react_agent(get_llm(), inventory_tools)

async def inventory_agent_node(state: AgentState) -> dict:
    """Inventory specialist parsing stock values and forecasting."""
    result = await inventory_bot.ainvoke({"messages": state["messages"]})
    return {"messages": result["messages"]}
