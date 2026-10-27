from src.schemas.intents import IntentClassification
from src.graph.state import AgentState
from src.core.llm import get_llm

def get_router_llm():
    return get_llm()

ROUTER_SYSTEM_PROMPT = """
You are an Intent Classifier for an Enterprise ERP System.
Analyze the user query and classify it into exactly one category:
- FINANCE: Overdue invoices, M-Pesa matching, account ledgers, payment collection.
- INVENTORY: Stock levels, warehouse transfers, item locations, stock-takes.
- PROCUREMENT: Vendor RFQs, purchase orders, reorder points, supplier balances.
- CRM: Customer contact updates, order history, client lookups, create quotations, confirm sales orders, deliveries, and invoicing.

Inject tenant-specific rules: {business_context}
"""

async def route_intent_node(state: AgentState) -> dict:
    prompt = state["messages"][-1].content
    tenant_id = state.get("tenant_id", "default")
    
    # Placeholder for actual RAG business context logic
    business_context = "No specific rules active."
    
    formatted_prompt = ROUTER_SYSTEM_PROMPT.format(business_context=business_context)
    
    llm = get_router_llm()
    structured_router = llm.with_structured_output(IntentClassification)
    
    result = await structured_router.ainvoke([
        {"role": "system", "content": formatted_prompt},
        {"role": "user", "content": prompt}
    ])
    
    return {
        "intent_category": result.category,
        "confidence_score": result.confidence
    }
