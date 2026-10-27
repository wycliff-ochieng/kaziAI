from typing import Annotated, List, Any
from typing_extensions import TypedDict
from langchain_core.messages import AIMessage, BaseMessage, HumanMessage, SystemMessage, ToolMessage
from langgraph.graph.message import add_messages
from langgraph.graph import StateGraph, END
from langgraph.prebuilt import ToolNode
from langgraph.checkpoint.memory import MemorySaver

from src.core.llm import get_llm
from src.core.logger import setup_logger
from src.graph.state import AgentState
from src.graph.guardrails import guardrail_node
from src.services.pre_router import pre_route
from src.services.session_memory import session_memory
from src.tools.backend_client import (
    get_customers_tool,
    get_products_tool,
    get_sale_orders_tool,
    create_sale_order_tool,
    confirm_sale_order_tool,
    deliver_sale_order_tool,
    create_invoice_tool,
    post_invoice_tool,
    get_vendors_tool,
    get_purchase_orders_tool,
    get_purchase_order_by_id_tool,
    create_rfq_tool,
    confirm_purchase_order_tool,
    receive_purchase_order_tool,
    create_vendor_bill_tool,
    post_vendor_bill_tool,
    cancel_purchase_order_tool,
    get_invoices_tool,
    count_records_tool
)

logger = setup_logger("workflow")

# Token-budget guards for small-context models (e.g. Groq on-demand 7k ITPM).
# History grows per thread_id in MemorySaver; without a cap every turn resends
# full tool outputs and trips 413 rate_limit_exceeded.
MAX_HISTORY_MESSAGES = 6
MAX_TOOL_CHARS = 2000


def _trim_messages(messages: List[BaseMessage]) -> List[BaseMessage]:
    """Sliding window: keep only the last N messages for the LLM call."""
    if not messages:
        return messages
    return messages[-MAX_HISTORY_MESSAGES:]


def _truncate_text(content: Any) -> str:
    text = content if isinstance(content, str) else str(content)
    if len(text) > MAX_TOOL_CHARS:
        return text[:MAX_TOOL_CHARS] + f"\n…[truncated {len(text) - MAX_TOOL_CHARS} chars, showing first {MAX_TOOL_CHARS}]"
    return text

SYSTEM_PROMPT = """You are an Odoo operations assistant.

CORE RULES
1. Use a tool for every factual claim about ERP data. If you did not see it in a
   tool result, do not state it.
2. Never invent IDs, counts, totals, statuses, or record names. Resolve names by
   calling the matching search tool first.
3. Read requests: call the narrowest list/search tool, pass every filter the user
   gave (customer_name, state, relationship, limit). Do not fetch everything and
   filter manually. "Unpaid" means payment_state=not_paid; "overdue" additionally
   means past its due date — do not treat them as the same filter.
4. Write requests: call the write tool to produce a preview. Write tools NEVER
   execute — they return a dry-run. Say what will happen and ask for approval.
   Never say "created", "confirmed", "delivered", "posted" on a preview.
5. Missing slot: ask ONE question for the missing piece. Do not invent a customer,
   product, quantity, price, or date.
6. Ambiguous name: the tool returns candidates. Show them and ask which one.
7. Compound requests ("create an order and deliver it"): call the create tool and
   set deliver_immediately=true. The system will expand the approved chain.
8. Quotations are sale orders in draft/sent state.
9. If nothing applies (analysis, forecasting, a resource you do not have), say so
   plainly. Do not substitute a nearby list.

STYLE
- Be brief. State the result, then one useful next step.
- Do not write tables; the UI renders structured rows from the tool result.
"""

def _last_human_text(messages):
    for m in reversed(messages or []):
        if isinstance(m, HumanMessage):
            return m.content if isinstance(m.content, str) else str(m.content)
    return ""


def build_ai_engine_graph():
    tools = [
        get_customers_tool, get_products_tool, get_sale_orders_tool, create_sale_order_tool,
        confirm_sale_order_tool, deliver_sale_order_tool, create_invoice_tool, post_invoice_tool,
        get_vendors_tool, get_purchase_orders_tool, get_purchase_order_by_id_tool, create_rfq_tool,
        confirm_purchase_order_tool, receive_purchase_order_tool, create_vendor_bill_tool,
        post_vendor_bill_tool, cancel_purchase_order_tool, get_invoices_tool, count_records_tool
    ]

    async def intent_router_node(state: AgentState):
        """Deterministic gate: conversational turns never reach the LLM/tools."""
        text = _last_human_text(state.get("messages", []))
        decision = pre_route(text)
        if decision is not None:
            return {
                "route": "conversational",
                "conversational_reply": decision.summary,
                "final_response_text": decision.summary,
            }
        return {"route": "erp"}

    async def converse_node(state: AgentState):
        reply = state.get("conversational_reply") or "How can I help with your Odoo data?"
        return {"messages": [AIMessage(content=reply)]}

    async def agent_node(state: AgentState):
        msgs = [SystemMessage(content=SYSTEM_PROMPT)] + _trim_messages(state["messages"])
        # Read the runtime config per turn so UI key/model changes take effect
        # without rebuilding or restarting the graph.
        response = await get_llm().bind_tools(tools).ainvoke(msgs)
        return {"messages": [response]}

    base_tool_node = ToolNode(tools)

    async def truncating_tool_node(state: AgentState):
        result = await base_tool_node.ainvoke(state)
        msgs_out = result.get("messages", []) if isinstance(result, dict) else []
        trimmed: List[BaseMessage] = []
        for m in msgs_out:
            if isinstance(m, ToolMessage):
                # Keep the full payload for the gateway (tables, IDs, counts);
                # only the copy sent to the small-context model is shortened.
                raw = m.content if isinstance(m.content, str) else str(m.content)
                m.additional_kwargs = {**(m.additional_kwargs or {}), "raw_content": raw}
                m.content = _truncate_text(raw)
            trimmed.append(m)
        return {"messages": trimmed}

    def should_continue(state: AgentState):
        last_message = state["messages"][-1]
        if hasattr(last_message, "tool_calls") and last_message.tool_calls:
            return "tools"
        return "guardrails"

    def route_after_router(state: AgentState):
        return state.get("route") or "erp"

    workflow = StateGraph(AgentState)
    workflow.add_node("intent_router", intent_router_node)
    workflow.add_node("converse", converse_node)
    workflow.add_node("agent", agent_node)
    workflow.add_node("tools", truncating_tool_node)
    workflow.add_node("guardrails", guardrail_node)

    workflow.set_entry_point("intent_router")
    workflow.add_conditional_edges(
        "intent_router", route_after_router, {"conversational": "converse", "erp": "agent"}
    )
    workflow.add_edge("converse", END)
    workflow.add_conditional_edges("agent", should_continue, ["tools", "guardrails"])
    workflow.add_edge("tools", "agent")
    workflow.add_edge("guardrails", END)

    checkpointer = MemorySaver()
    return workflow.compile(checkpointer=checkpointer)

compiled_ai_engine = build_ai_engine_graph()
