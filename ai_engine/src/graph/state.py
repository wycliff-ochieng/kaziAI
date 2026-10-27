from typing import Annotated, List, Dict, Any, Optional
from typing_extensions import TypedDict
from langchain_core.messages import BaseMessage
from langgraph.graph.message import add_messages
from pydantic import BaseModel

class DryRunPreview(BaseModel):
    action_type: str
    target_module: str
    impact_summary: str
    affected_records_count: int
    financial_value_kes: float
    raw_payload: Dict[str, Any]
    requires_human_approval: bool = True

class AgentState(TypedDict):
    # Context & Tenant Metadata
    tenant_id: str
    user_id: str
    channel: str  # "WHATSAPP", "WEB"
    
    # Message Memory
    messages: Annotated[List[BaseMessage], add_messages]
    
    # Classification & Routing State
    intent_category: Optional[str]  # "FINANCE", "INVENTORY", "PROCUREMENT", "CRM"
    confidence_score: float
    #: Deterministic pre-router decision: "conversational" or "erp".
    route: Optional[str]
    conversational_reply: Optional[str]
    
    # Tool Execution State
    execution_plan: List[Dict[str, Any]]  # List of steps to execute sequentially
    pending_tool_calls: List[Dict[str, Any]]
    tool_results: List[Dict[str, Any]]

    # Slot-filling drafts (Dialogue State Tracking across turns)
    draft_customer_name: Optional[str]
    draft_customer_id: Optional[str]
    draft_product_name: Optional[str]
    draft_product_id: Optional[str]
    draft_quantity: Optional[float]
    
    # Safety & Output
    dry_run_preview: Optional[DryRunPreview]
    final_response_text: Optional[str]
    error: Optional[str]
