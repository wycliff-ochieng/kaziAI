from langgraph.prebuilt import create_react_agent
from src.tools.backend_client import get_sale_orders_tool
from src.core.llm import get_llm
from src.graph.state import AgentState

finance_tools = [get_sale_orders_tool] # Additional mock tools: reconcile_mpesa, fetch_overdue_invoices

finance_bot = create_react_agent(get_llm(), finance_tools)

async def finance_agent_node(state: AgentState) -> dict:
    """Finance specialist agent executing accounting and payment tools."""
    result = await finance_bot.ainvoke({"messages": state["messages"]})
    return {"messages": result["messages"]}
