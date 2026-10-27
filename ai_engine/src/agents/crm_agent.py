from langgraph.prebuilt import create_react_agent
from src.tools.backend_client import get_customers_tool
from src.core.llm import get_llm
from src.graph.state import AgentState

crm_tools = [get_customers_tool] 

crm_bot = create_react_agent(get_llm(), crm_tools)

async def crm_agent_node(state: AgentState) -> dict:
    """CRM specialist handling accounts and sales communications."""
    result = await crm_bot.ainvoke({"messages": state["messages"]})
    return {"messages": result["messages"]}
