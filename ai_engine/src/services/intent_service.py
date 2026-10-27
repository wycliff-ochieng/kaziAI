import re
import time
from typing import Any, Dict, List, Optional, Tuple, cast

from src.core.config import settings
from src.core.llm import get_llm
from src.core.logger import setup_logger
from src.schemas.intents import (
    CONVERSATIONAL_INTENTS,
    HIGH_RISK_INTENTS,
    READ_INTENTS,
    WRITE_INTENTS,
    ActionType,
    IntentParameters,
    IntentType,
    ParsedIntentResponse,
    ParseIntentRequest,
    ValidationRequirement,
)
from src.services import filter_compiler
from src.services.pre_router import pre_route
from src.services.session_memory import session_memory

logger = setup_logger("intent_service")


SYSTEM_PROMPT = """You are the intent and entity extractor for an Odoo ERP agent.
Read the user's message (and the session frame) and return ONE structured object.

## STEP 1 — IS IT EVEN AN ERP REQUEST?
- Greetings, thanks, "who are you", "what can you do", apologies, corrections and
  idle chat -> intent_type = CONVERSATION, action_type = CONVERSATION, no entities.
- If the request is too vague to act on (no resource and no clear verb)
  -> intent_type = CLARIFY and put a short question in summary.
- Never turn chit-chat into a data request. "hello" is NOT a customer lookup.

## STEP 2 — PICK THE OPERATION
Read:    GET_CUSTOMERS, GET_CUSTOMER_DETAILS, GET_PRODUCTS, GET_PRODUCT_DETAILS,
         GET_SALE_ORDERS, GET_SALE_ORDER_DETAILS, GET_INVOICES, GET_OVERDUE_INVOICES,
         GET_VENDORS, GET_PURCHASE_ORDERS, GET_PURCHASE_ORDER_DETAILS
Count:   COUNT_RECORDS  (only when the user asks "how many"; never for "show/list")
Write:   CREATE_QUOTATION, CREATE_SALE_ORDER, CONFIRM_SALE_ORDER, CANCEL_SALE_ORDER,
         DELIVER_SALE_ORDER, CREATE_INVOICE, POST_INVOICE, CREATE_CREDIT_NOTE,
         CREATE_RFQ, CONFIRM_PURCHASE_ORDER, RECEIVE_PURCHASE_ORDER,
         CREATE_VENDOR_BILL, POST_VENDOR_BILL, CANCEL_PURCHASE_ORDER
Other:   SUBMIT_ETIMS, PRINT_DOCUMENT, APPROVE_ACTION, UNKNOWN

LIST vs COUNT are different operations. "show 10 customers" is GET_CUSTOMERS with
limit=10. "how many customers are there" is COUNT_RECORDS with resource="customers".

## STEP 2b — REFUSE WHAT YOU CANNOT DO (do not substitute a nearby list)
You can only perform the operations listed above, over these resources:
  customers, vendors, products, orders (sale), purchase_orders, invoices, deliveries.
If the request needs analysis, ranking, forecasting, comparison, or a resource/field
that is not listed — e.g. "which product is moving slowest", "who is my best
salesperson", "what is my profit margin", "forecast next month" — DO NOT return the
closest list. Return intent_type=CLARIFY with a short summary that states what you
cannot do and offers a nearby thing you CAN do. Using UNKNOWN is also acceptable.
Returning a generic list for an analytical question is a failure.

## STEP 3 — EXTRACT ENTITIES (STRICT BOUNDARIES)
- customer_name / vendor_name / product_name are CLEAN names only (1-4 words).
  Strip conjunctions, verbs, quantities, prices and any trailing instruction.
  Input: "create a sale order for eslam mohamed and add bl Serum and deliver it"
  customer_name = "Eslam Mohamed"   NOT the whole sentence.
- Put each ordered item into lines[] as {product_name, quantity}.
- NEVER invent IDs. Leave *_id empty unless the user typed a literal ID/reference.
- state: normalise to Odoo states (draft, sent, sale, done, cancel, posted, paid...).

## STEP 4 — QUERY IR (filters, limit, relationship)
- resource: customers | vendors | products | orders | purchase_orders | invoices | deliveries
- limit: set ONLY when the user bounds the result ("first 10", "top 5"). Otherwise 0.
- filters[]: one object per constraint, using ONLY these fields:
  customers: id, name, email, phone, balance, credit_limit, sale_order_count, invoice_count, city, create_date
  vendors:   id, name, email, phone, balance
  products:  id, name, default_code, list_price, qty_available
  orders:    id, name, partner_id, state, amount_total, date_order, invoice_status
  purchase_orders: id, name, partner_id, state, amount_total, date_order
  invoices:  id, name, partner_id, state, payment_state, move_type, amount_total, amount_residual, invoice_date, due_date
  deliveries: id, name, sale_id, partner_id, state, picking_type_code
  operators: =, !=, >, >=, <, <=, ilike, like, not ilike, in, not in
  Examples:
   "invoices in paid state"          -> resource=invoices, filters=[{state,=,posted},{payment_state,=,paid}]
   "customers with >1 sale order"    -> resource=customers, filters=[{sale_order_count,>,1}]
   "products with zero stock"        -> resource=products, filters=[{qty_available,<=,0}]
- relationship: use when the user asks for records RELATED to a named document:
   "deliveries for S00009"           -> resource=deliveries, relationship=deliveries_for_order, order_id=S00009
   "invoices for SO123"              -> resource=invoices, relationship=invoices_for_order, order_id=SO123
   "orders for Eslam"                -> resource=orders, relationship=orders_for_customer, customer_name=Eslam
  Do NOT return the parent document when the user asked for its children.

## STEP 5 — MULTI-STEP CHAINS
If the request contains an ordered sequence of actions, set intent_type to the
FIRST action and list the rest in steps[] (each with intent_type + parameters).
Example: "create a sale order for eslam, add bl Serum and deliver it"
  intent_type=CREATE_SALE_ORDER, steps=[{CONFIRM_SALE_ORDER},{DELIVER_SALE_ORDER}]

## SAFETY
- dry_run stays true unless the user explicitly says "execute", "live", "real",
  "commit", "for real" or supplies an approval token.
- Set confidence honestly (0-1). Use UNKNOWN only when nothing else fits.

## ACTION TYPE & RISK (the engine re-derives these, but stay consistent)
- READ: GET_*, COUNT_RECORDS, PRINT_DOCUMENT.
- WRITE: CREATE_*, CONFIRM_*, CANCEL_*, DELIVER_*, POST_*, RECEIVE_*, SUBMIT_ETIMS.
- CONVERSATION: CONVERSATION, CLARIFY, AFFIRM, REJECT.
- risk_level HIGH for cancel/deliver/credit-note/eTIMS/post-bill; MEDIUM for
  creates and confirms; LOW for reads.
- For a single-item create, fill BOTH lines=[{product_name,quantity}] AND the
  top-level product_name + quantity (older clients read the flat slots).
"""


_RETRY_HINT_RE = re.compile(r"try again in ([0-9.]+)s", re.IGNORECASE)


def _is_rate_limit(exc: Exception) -> bool:
    text = str(exc)
    return "429" in text or "rate_limit_exceeded" in text or "Request too large" in text or "OTPM" in text


def _invoke_with_backoff(llm, messages: List[Tuple[str, str]]) -> Any:
    """Call the model, backing off on the Groq on-demand output-token budget.

    The on-demand tier caps output tokens per minute (~1000), so a short burst
    of turns is rejected even when each individual request is tiny. We honour
    the provider's own ``try again in Ns`` hint before giving up.
    """
    attempts = max(1, settings.LLM_MAX_RETRIES_ON_429)
    last_exc: Optional[Exception] = None
    for attempt in range(attempts):
        try:
            return llm.invoke(messages)
        except Exception as exc:  # noqa: BLE001 - inspect provider message
            last_exc = exc
            if not _is_rate_limit(exc) or attempt == attempts - 1:
                raise
            hint = _RETRY_HINT_RE.search(str(exc))
            wait = float(hint.group(1)) if hint else 2.0 * (2**attempt)
            wait = min(wait + 1.0, settings.LLM_429_MAX_BACKOFF_SECONDS)
            logger.warning("rate limited (attempt %d/%d); retrying in %.1fs", attempt + 1, attempts, wait)
            time.sleep(wait)
    raise last_exc if last_exc else RuntimeError("LLM invocation failed")


def _fallback(reason: str) -> ParsedIntentResponse:
    """Graceful degradation: never surface a raw 500 for a model-capacity error."""
    return ParsedIntentResponse(
        intent_type=IntentType.CLARIFY,
        confidence=0.0,
        action_type=ActionType.CONVERSATION,
        summary=(
            "I couldn't reach my language model just now, so I didn't run anything. "
            "Please try again in a moment."
        ),
        parameters=IntentParameters(dry_run=False),
        validation=ValidationRequirement(
            requires_user_approval=False,
            risk_level="LOW",
            impact_summary=reason,
        ),
    )


#: Deterministic approval questions so the frontend always has a prompt/card
#: even when the model omits one. Kept short and action-specific.
_APPROVAL_PROMPTS: Dict[IntentType, str] = {
    IntentType.CREATE_QUOTATION: "Would you like to confirm and generate this Quotation",
    IntentType.CREATE_SALE_ORDER: "Do you want to confirm and validate the creation of this Sale Order",
    IntentType.CONFIRM_SALE_ORDER: "Confirming this Sale Order will lock inventory stock. Do you want to proceed",
    IntentType.DELIVER_SALE_ORDER: "Are you sure you want to validate and process inventory delivery",
    IntentType.CANCEL_SALE_ORDER: "CAUTION: cancelling this Sale Order voids it and releases stock. Do you confirm",
    IntentType.CREATE_CREDIT_NOTE: "HIGH IMPACT: this Credit Note refunds funds. Do you confirm",
    IntentType.SUBMIT_ETIMS: "Fiscalising this invoice transmits official tax data to KRA eTIMS. Do you confirm",
    IntentType.CREATE_RFQ: "Do you want to draft this RFQ",
    IntentType.CONFIRM_PURCHASE_ORDER: "Confirming this PO commits spend. Do you want to proceed",
    IntentType.RECEIVE_PURCHASE_ORDER: "Receive all lines of this PO into stock",
    IntentType.CREATE_VENDOR_BILL: "Do you want to draft this vendor bill",
    IntentType.POST_VENDOR_BILL: "Do you want to post this vendor bill",
    IntentType.CANCEL_PURCHASE_ORDER: "Do you want to cancel this draft RFQ",
}


def _default_approval_prompt(response: ParsedIntentResponse) -> str:
    params = response.parameters
    base = _APPROVAL_PROMPTS.get(response.intent_type, "Do you want to confirm this action")
    if params.customer_name:
        return f"{base} for {params.customer_name}?"
    if params.order_id:
        return f"{base} {params.order_id}?"
    if params.vendor_name:
        return f"{base} for {params.vendor_name}?"
    return f"{base}?"


class IntentService:
    """Parses natural language into canonical ERP intents, entities and query IR."""

    def parse_intent(self, request: ParseIntentRequest) -> ParsedIntentResponse:
        prompt = (request.prompt or "").strip()
        thread_id = request.thread_id or "default_thread"
        tenant_id = request.tenant_id or "default_tenant"

        # 1. Deterministic conversational guard — no LLM, no ERP, no token spend.
        fast = pre_route(prompt)
        if fast is not None:
            return fast

        # 2. Session frame gives the model grounded memory instead of raw dumps.
        frame = session_memory.describe(thread_id, tenant_id)
        context_hint = self._context_hint(request.context)

        llm = get_llm().with_structured_output(ParsedIntentResponse)

        user_block = (
            f"SESSION FRAME:\n{frame}\n\n"
            f"EXTRA CONTEXT:\n{context_hint}\n\n"
            f"USER MESSAGE:\n{prompt}"
        )

        try:
            response = cast(ParsedIntentResponse, _invoke_with_backoff(
                llm, [("system", SYSTEM_PROMPT), ("human", user_block)]
            ))
        except Exception as exc:  # rate limits, timeouts, provider outages
            logger.warning("intent parse degraded: %s", exc)
            return _fallback(str(exc))

        response = self._post_process(response, thread_id, tenant_id)
        return response

    # -- helpers ---------------------------------------------------------------

    @staticmethod
    def _context_hint(context: Optional[Dict[str, Any]]) -> str:
        if not context:
            return "(none)"
        # Keep it small: cap values so a caller can't blow the token budget.
        parts = []
        for key, value in list(context.items())[:8]:
            text = str(value)
            if len(text) > 200:
                text = text[:200] + "…"
            parts.append(f"{key}={text}")
        return "; ".join(parts) or "(none)"

    @staticmethod
    def _enforce_taxonomy(response: ParsedIntentResponse) -> ParsedIntentResponse:
        """Derive action_type and the risk floor from the intent, not the model.

        Safety classification must not depend on an LLM guessing; the intent
        taxonomy already encodes it.
        """
        intent = response.intent_type
        if intent in CONVERSATIONAL_INTENTS:
            response.action_type = ActionType.CONVERSATION
        elif intent in WRITE_INTENTS:
            response.action_type = ActionType.WRITE
        elif intent in READ_INTENTS:
            response.action_type = ActionType.READ

        risk = (response.validation.risk_level or "MEDIUM").upper()
        if intent in HIGH_RISK_INTENTS:
            risk = "HIGH"
        elif intent in READ_INTENTS or intent in CONVERSATIONAL_INTENTS:
            risk = "LOW"
        elif risk not in ("LOW", "MEDIUM", "HIGH"):
            risk = "MEDIUM"
        response.validation.risk_level = risk
        response.validation.requires_user_approval = intent in WRITE_INTENTS
        if intent in WRITE_INTENTS and not (response.validation.approval_prompt or "").strip():
            response.validation.approval_prompt = _default_approval_prompt(response)
        return response

    def _post_process(
        self, response: ParsedIntentResponse, thread_id: str, tenant_id: str
    ) -> ParsedIntentResponse:
        response = self._enforce_taxonomy(response)
        params = response.parameters or IntentParameters()

        # Guard against hallucinated IDs (unchanged behaviour, widened coverage).
        for field_name in ("customer_id", "vendor_id", "order_id", "invoice_id", "quotation_id", "product_id", "bill_id"):
            value = getattr(params, field_name, None)
            if isinstance(value, str) and value and ("SIMULATED" in value.upper() or value.upper().startswith("ID_")):
                setattr(params, field_name, None)

        # Mirror a single order line into the flat slots for older clients.
        if len(params.lines) == 1:
            line = params.lines[0]
            if not params.product_name and line.product_name:
                params.product_name = line.product_name
            if not params.product_id and line.product_id:
                params.product_id = line.product_id
            if (params.quantity or 0.0) <= 0 and line.quantity:
                params.quantity = line.quantity

        # Compile allowlisted filters into a validated Odoo domain.
        if params.filters:
            resource = params.resource or response.intent_type.value.lower()
            compiled = filter_compiler.compile_filters(resource, params.filters)
            params.domain = compiled.domain
            params.filters = [f for f in params.filters if f.field in filter_compiler.allowed_fields(resource)]
            if compiled.errors:
                note = "; ".join(compiled.errors[:3])
                existing = response.validation.impact_summary or ""
                response.validation.impact_summary = (existing + " " + f"Ignored invalid filters: {note}.").strip()
                response.confidence = min(response.confidence, 0.6)

        # Ground the session frame in what this turn establishes.
        if response.action_type in (ActionType.READ, ActionType.SIMULATION):
            session_memory.record_query(
                thread_id=thread_id,
                tenant_id=tenant_id,
                intent_type=response.intent_type.value,
                resource=params.resource or "",
                filters=[f.model_dump() for f in params.filters],
                domain=params.domain,
                summary=response.summary,
            )
        elif response.action_type == ActionType.WRITE and params.dry_run:
            session_memory.set_pending(thread_id, tenant_id, response.intent_type.value, response.summary)

        response.parameters = params
        return response


intent_service = IntentService()
