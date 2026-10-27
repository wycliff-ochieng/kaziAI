from enum import Enum
from typing import Optional, List, Dict, Any
from pydantic import BaseModel, Field


class ActionType(str, Enum):
    READ = "READ"
    WRITE = "WRITE"
    SIMULATION = "SIMULATION"
    CONVERSATION = "CONVERSATION"


class IntentType(str, Enum):
    # Conversational (never touches the ERP)
    CONVERSATION = "CONVERSATION"
    CLARIFY = "CLARIFY"

    # Customer Intents
    GET_CUSTOMERS = "GET_CUSTOMERS"
    GET_CUSTOMER_DETAILS = "GET_CUSTOMER_DETAILS"

    # Product Intents
    GET_PRODUCTS = "GET_PRODUCTS"
    GET_PRODUCT_DETAILS = "GET_PRODUCT_DETAILS"

    # Quotation & Sales Order Intents
    CREATE_QUOTATION = "CREATE_QUOTATION"
    SEND_QUOTATION = "SEND_QUOTATION"
    CONVERT_QUOTATION_TO_ORDER = "CONVERT_QUOTATION_TO_ORDER"
    CREATE_SALE_ORDER = "CREATE_SALE_ORDER"
    CONFIRM_SALE_ORDER = "CONFIRM_SALE_ORDER"
    CANCEL_SALE_ORDER = "CANCEL_SALE_ORDER"
    DELIVER_SALE_ORDER = "DELIVER_SALE_ORDER"
    GET_SALE_ORDERS = "GET_SALE_ORDERS"
    GET_SALE_ORDER_DETAILS = "GET_SALE_ORDER_DETAILS"

    # Invoice & Credit Note Intents
    CREATE_INVOICE = "CREATE_INVOICE"
    POST_INVOICE = "POST_INVOICE"
    CREATE_CREDIT_NOTE = "CREATE_CREDIT_NOTE"
    GET_INVOICES = "GET_INVOICES"
    GET_OVERDUE_INVOICES = "GET_OVERDUE_INVOICES"

    # Aggregate (list vs count must never be conflated)
    COUNT_RECORDS = "COUNT_RECORDS"

    # eTIMS & KRA Fiscalization Compliance
    SUBMIT_ETIMS = "SUBMIT_ETIMS"

    # Document Printing / Rendering
    PRINT_DOCUMENT = "PRINT_DOCUMENT"

    # Approval / User Validation Flow
    APPROVE_ACTION = "APPROVE_ACTION"

    # Conversational approval (pending dry-run follow-ups: "okay", "confirmed", "go on" / rejections)
    AFFIRM = "AFFIRM"
    REJECT = "REJECT"

    # Vendor & Purchase Order Intents (canonical; ERP mapping lives in Go adapters)
    GET_VENDORS = "GET_VENDORS"
    GET_PURCHASE_ORDERS = "GET_PURCHASE_ORDERS"
    GET_PURCHASE_ORDER_DETAILS = "GET_PURCHASE_ORDER_DETAILS"
    CREATE_RFQ = "CREATE_RFQ"
    CONFIRM_PURCHASE_ORDER = "CONFIRM_PURCHASE_ORDER"
    CANCEL_PURCHASE_ORDER = "CANCEL_PURCHASE_ORDER"
    RECEIVE_PURCHASE_ORDER = "RECEIVE_PURCHASE_ORDER"
    CREATE_VENDOR_BILL = "CREATE_VENDOR_BILL"
    POST_VENDOR_BILL = "POST_VENDOR_BILL"

    # Fallback
    UNKNOWN = "UNKNOWN"


#: Intents that are pure dialogue and must never reach ERP execution.
CONVERSATIONAL_INTENTS = {
    IntentType.CONVERSATION,
    IntentType.CLARIFY,
    IntentType.AFFIRM,
    IntentType.REJECT,
}

#: Read-only intents (used to enforce action_type deterministically).
READ_INTENTS = {
    IntentType.GET_CUSTOMERS,
    IntentType.GET_CUSTOMER_DETAILS,
    IntentType.GET_PRODUCTS,
    IntentType.GET_PRODUCT_DETAILS,
    IntentType.GET_SALE_ORDERS,
    IntentType.GET_SALE_ORDER_DETAILS,
    IntentType.GET_INVOICES,
    IntentType.GET_OVERDUE_INVOICES,
    IntentType.GET_VENDORS,
    IntentType.GET_PURCHASE_ORDERS,
    IntentType.GET_PURCHASE_ORDER_DETAILS,
    IntentType.COUNT_RECORDS,
    IntentType.PRINT_DOCUMENT,
}

#: Mutating intents (used to enforce action_type deterministically).
WRITE_INTENTS = {
    IntentType.CREATE_QUOTATION,
    IntentType.SEND_QUOTATION,
    IntentType.CONVERT_QUOTATION_TO_ORDER,
    IntentType.CREATE_SALE_ORDER,
    IntentType.CONFIRM_SALE_ORDER,
    IntentType.CANCEL_SALE_ORDER,
    IntentType.DELIVER_SALE_ORDER,
    IntentType.CREATE_INVOICE,
    IntentType.POST_INVOICE,
    IntentType.CREATE_CREDIT_NOTE,
    IntentType.SUBMIT_ETIMS,
    IntentType.CREATE_RFQ,
    IntentType.CONFIRM_PURCHASE_ORDER,
    IntentType.CANCEL_PURCHASE_ORDER,
    IntentType.RECEIVE_PURCHASE_ORDER,
    IntentType.CREATE_VENDOR_BILL,
    IntentType.POST_VENDOR_BILL,
    IntentType.APPROVE_ACTION,
}

#: Intents whose risk floor is HIGH (irreversible / money / stock).
HIGH_RISK_INTENTS = {
    IntentType.CANCEL_SALE_ORDER,
    IntentType.CANCEL_PURCHASE_ORDER,
    IntentType.DELIVER_SALE_ORDER,
    IntentType.CREATE_CREDIT_NOTE,
    IntentType.SUBMIT_ETIMS,
    IntentType.POST_VENDOR_BILL,
}


class OrderLinePayload(BaseModel):
    product_name: Optional[str] = None
    product_id: Optional[str] = None
    quantity: float = 1.0
    unit_price: float = 0.0


class QueryFilter(BaseModel):
    """One natural-language constraint, already split out of the prompt.

    Example: "customers with more than one sale order" ->
        field="sale_order_count", operator=">", value="1"
    The value stays a string here; the compiler casts it against the
    resource's field allowlist before it becomes an Odoo domain tuple.
    """

    field: str
    operator: str = "="
    value: Any = None


class IntentParameters(BaseModel):
    # Entity slots (names are resolved downstream; IDs are never invented)
    customer_name: Optional[str] = None
    customer_id: Optional[str] = None
    vendor_name: Optional[str] = None
    vendor_id: Optional[str] = None
    bill_id: Optional[str] = None
    product_name: Optional[str] = None
    product_id: Optional[str] = None
    order_id: Optional[str] = None
    invoice_id: Optional[str] = None
    quotation_id: Optional[str] = None
    reason: Optional[str] = None
    document_type: Optional[str] = None
    format: str = "pdf"
    search: Optional[str] = None
    state: Optional[str] = None
    quantity: float = 1.0
    unit_price: float = 0.0
    amount: float = 0.0
    lines: List[OrderLinePayload] = Field(default_factory=list)
    note: Optional[str] = None

    # Query IR (list/get/search/count semantics)
    resource: Optional[str] = None
    relationship: Optional[str] = None
    filters: List[QueryFilter] = Field(default_factory=list)
    limit: int = 0
    offset: int = 0
    sort: Optional[str] = None
    #: Compiled, allowlisted Odoo domain (filled by the engine, read by the backend).
    domain: List[Any] = Field(default_factory=list)

    dry_run: bool = True
    approval_token: Optional[str] = None


class IntentStep(BaseModel):
    """One link in a multi-intent chain. Primary intent stays at top level."""

    intent_type: IntentType
    summary: str = ""
    parameters: IntentParameters = Field(default_factory=IntentParameters)


class ValidationRequirement(BaseModel):
    requires_user_approval: bool = True
    risk_level: str = "MEDIUM"  # LOW, MEDIUM, HIGH
    approval_prompt: str = ""  # Explicit human query asking user to validate
    impact_summary: str = ""


class ParseIntentRequest(BaseModel):
    prompt: str = Field(..., description="Natural language request from user")
    thread_id: Optional[str] = Field(default="", description="Thread ID for conversational memory")
    tenant_id: Optional[str] = Field(default="default_tenant", description="Tenant ID")
    conversation_id: Optional[str] = Field(default="default", description="Conversation ID scoped to the tenant")
    erp_type: Optional[str] = Field(default="", description="Tenant ERP type (MOCK/ODOO) for tool routing")
    context: Optional[Dict[str, Any]] = Field(default_factory=dict, description="Session context")


class ParsedIntentResponse(BaseModel):
    intent_type: IntentType
    confidence: float = 0.5
    action_type: ActionType = ActionType.READ
    summary: str = ""
    parameters: IntentParameters = Field(default_factory=IntentParameters)
    validation: ValidationRequirement = Field(default_factory=ValidationRequirement)
    #: Ordered follow-on actions for compound requests ("create X and deliver it").
    steps: List[IntentStep] = Field(default_factory=list)


class IntentClassification(BaseModel):
    category: str = Field(description="The functional area category: FINANCE, INVENTORY, PROCUREMENT, or CRM.")
    confidence: float = Field(description="Confidence level between 0 and 1.")
