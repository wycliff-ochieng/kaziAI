from src.graph.state import AgentState
from src.tools.backend_client import (
    get_customers_tool,
    get_products_tool,
    get_sale_orders_tool,
    create_sale_order_tool,
    confirm_sale_order_tool,
    deliver_sale_order_tool,
    create_invoice_tool,
    post_invoice_tool
)
from langchain_core.messages import AIMessage
import traceback

sales_tools = [
    get_customers_tool,
    get_products_tool,
    get_sale_orders_tool,
    create_sale_order_tool,
    confirm_sale_order_tool,
    deliver_sale_order_tool,
    create_invoice_tool,
    post_invoice_tool
]

def get_tool_by_name(name: str):
    for t in sales_tools:
        if t.name == name:
            return t
    return None

async def executor_node(state: AgentState) -> dict:
    """Executes the next step in the execution plan."""
    plan = state.get("execution_plan", [])
    if not plan:
        return {}

    # Pop the first step
    step = plan.pop(0)
    tool_name = step.get("tool")
    kwargs = step.get("kwargs", {})
    
    if not tool_name:
        return {"error": "Tool name is missing in execution step", "execution_plan": []}
        
    tool = get_tool_by_name(str(tool_name))
    if not tool:
        # Halt on error
        return {"error": f"Tool {tool_name} not found", "execution_plan": []}

    try:
        # Execute tool
        result = await tool.ainvoke(kwargs)
        
        tool_results = state.get("tool_results", [])
        tool_results.append({
            "tool": tool_name,
            "result": result
        })
        
        return {
            "execution_plan": plan, 
            "tool_results": tool_results,
            "messages": [AIMessage(content=f"Executed {tool_name}: {result}")]
        }
    except Exception as e:
        error_msg = f"Tool {tool_name} failed: {str(e)}\n{traceback.format_exc()}"
        return {
            "error": error_msg,
            "execution_plan": [], # Empty the plan to halt execution
        }
