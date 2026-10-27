from langgraph.prebuilt import create_react_agent
from src.tools.backend_client import (
    get_vendors_tool,
    get_products_tool,
    get_purchase_orders_tool,
    get_purchase_order_by_id_tool,
    create_rfq_tool,
    confirm_purchase_order_tool,
    receive_purchase_order_tool,
    create_vendor_bill_tool,
    post_vendor_bill_tool,
    cancel_purchase_order_tool,
)
from src.core.llm import get_llm
from src.graph.state import AgentState

procurement_tools = [
    get_vendors_tool,
    get_products_tool,
    get_purchase_orders_tool,
    get_purchase_order_by_id_tool,
    create_rfq_tool,
    confirm_purchase_order_tool,
    receive_purchase_order_tool,
    create_vendor_bill_tool,
    post_vendor_bill_tool,
    cancel_purchase_order_tool,
]

PURCHASE_SYSTEM_PROMPT = """You are an expert Purchase Operations AI over a Universal ERP Gateway.
Your goal is to flawlessly execute the standard purchase workflow:
1. Entity Resolution: Before creating RFQs, ALWAYS use get_vendors_tool and get_products_tool to search and find the exact vendor_id and product_id. Never guess IDs! Search-only in v1; never auto-create vendors/products.
2. RFQ Draft (single vendor, multi-line via repeated items): Use create_rfq_tool to build the initial draft RFQ. Draft auto-executes (low risk).
3. Verify-Before-Act: Before ANY confirm/receive/bill/cancel on an existing order, ALWAYS call get_purchase_order_by_id_tool to check canonical state (draft/rfq -> purchase -> done, receipt pending/fully_received, billing no/to_bill/billed). Act only if the transition is legal; otherwise explain and stop.
4. Confirmation (Purchase Order): Use confirm_purchase_order_tool with the order ID. Defaults to dry_run preview; requires explicit user approval to execute.
5. Receipt Validation: Use receive_purchase_order_tool for FULL receipt only. Partials, backorders, and over-receipts are rejected in v1.
6. Vendor Bill: Use create_vendor_bill_tool ONLY on explicit user request — never automatically after receipt. The bill is auto-copied from PO lines (no price edits in v1). Then post_vendor_bill_tool posts the journal only (no payment execution in v1).
7. Cancel: cancel_purchase_order_tool works on draft/RFQ state only; confirmed POs cannot be cancelled in v1. Gated by approval.

Autonomy: run the full chain by default (pausing for approval at each gate), stop early when the user says so (e.g. "stop at receipt"), and resume from any stage when given an order ID (verify state first, then continue). Provide clear status updates at each stage.
"""

from langchain_core.messages import SystemMessage

procurement_bot = create_react_agent(get_llm(), procurement_tools)


async def procurement_agent_node(state: AgentState) -> dict:
    """Procurement specialist for vendors, RFQs, POs, receipts, and vendor bills."""
    messages = [SystemMessage(content=PURCHASE_SYSTEM_PROMPT)] + state["messages"]
    result = await procurement_bot.ainvoke({"messages": messages})
    return {"messages": result["messages"]}
