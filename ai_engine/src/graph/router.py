from langchain_openai import ChatOpenAI
from src.schemas.intents import IntentClassification
from src.graph.state import AgentState
from src.core.config import settings

def get_router_llm():
    return ChatOpenAI(
        model=settings.LLM_MODEL, 
        temperature=0, 
        api_key=settings.LLM_API_KEY or "sk-dummy",
        base_url=settings.LLM_BASE_URL
    )

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
