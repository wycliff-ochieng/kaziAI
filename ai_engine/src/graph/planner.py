import json
from typing import Any, Dict, List
from langchain_core.messages import SystemMessage, HumanMessage
from src.core.llm import get_llm
from src.graph.state import AgentState

PLANNER_SYSTEM_PROMPT = """You are an ERP Sales Operations Planner.
Your goal is to break down the user's natural language request into a precise execution plan.
You must output ONLY valid JSON representing an array of ordered tool calls.
Do not output markdown formatting like ```json.
Example output:
[
  {"tool": "get_customers_tool", "kwargs": {"search": "Azba"}},
  {"tool": "get_products_tool", "kwargs": {"search": "Widget"}},
  {"tool": "create_sale_order_tool", "kwargs": {"customer_id": "{customer_id-from-step1}", "product_id": "{product_id-from-step2}", "quantity": 10}}
]
In kwargs, you can use placeholders if the data depends on prev steps.
"""

async def planner_node(state: AgentState) -> dict:
    """Takes the conversation history and generates an execution plan."""
    llm = get_llm()
    # Simple JSON generation for the plan
    messages = [SystemMessage(content=PLANNER_SYSTEM_PROMPT)] + state["messages"]
    result = await llm.ainvoke(messages)
    
    try:
        plan_text = str(result.content).strip()
        if plan_text.startswith("```json"):
            plan_text = plan_text[7:-3]
        plan = json.loads(plan_text)
    except Exception as e:
        # Fallback empty plan on failure
        plan = []
        
    return {"execution_plan": plan}
