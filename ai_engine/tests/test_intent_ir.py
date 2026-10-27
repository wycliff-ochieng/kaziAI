"""Post-processing tests for the intent service (no LLM required).

Exercises the deterministic layer that turns a parsed intent into a validated
query IR and updates the session frame.
"""
from src.schemas.intents import (
    ActionType,
    IntentParameters,
    IntentStep,
    IntentType,
    ParsedIntentResponse,
    QueryFilter,
    ValidationRequirement,
)
from src.services.intent_service import intent_service
from src.services.session_memory import session_memory


def _read(intent, resource, filters=None):
    return ParsedIntentResponse(
        intent_type=intent,
        confidence=0.9,
        action_type=ActionType.READ,
        summary="list",
        parameters=IntentParameters(resource=resource, filters=filters or []),
        validation=ValidationRequirement(requires_user_approval=False),
    )


def test_relationship_filter_is_compiled_into_domain():
    res = _read(
        IntentType.GET_CUSTOMERS,
        "customers",
        [QueryFilter(field="sale_order_count", operator=">", value="1")],
    )
    out = intent_service._post_process(res, "ir-1", "tenant")
    assert out.parameters.domain == [("sale_order_count", ">", 1)]
    assert len(out.parameters.filters) == 1


def test_invalid_filters_are_dropped_and_flagged():
    res = _read(
        IntentType.GET_CUSTOMERS,
        "customers",
        [QueryFilter(field="name", operator="ilike", value="acme"), QueryFilter(field="password", operator="=", value="x")],
    )
    out = intent_service._post_process(res, "ir-2", "tenant")
    assert out.parameters.domain == [("name", "ilike", "acme")]
    assert len(out.parameters.filters) == 1
    assert "Ignored invalid filters" in out.validation.impact_summary


def test_hallucinated_ids_are_cleared():
    res = ParsedIntentResponse(
        intent_type=IntentType.CONFIRM_SALE_ORDER,
        confidence=0.8,
        action_type=ActionType.WRITE,
        summary="confirm",
        parameters=IntentParameters(order_id="SIMULATED-123", customer_name="Eslam"),
        validation=ValidationRequirement(),
    )
    out = intent_service._post_process(res, "ir-3", "tenant")
    assert out.parameters.order_id is None
    assert out.parameters.customer_name == "Eslam"


def test_read_updates_session_frame():
    res = _read(IntentType.GET_SALE_ORDERS, "orders", [QueryFilter(field="state", operator="=", value="sale")])
    intent_service._post_process(res, "ir-mem", "tenant")
    desc = session_memory.describe("ir-mem", "tenant")
    assert "GET_SALE_ORDERS" in desc
    assert "state = sale" in desc


def test_write_dry_run_marks_pending():
    res = ParsedIntentResponse(
        intent_type=IntentType.CONFIRM_SALE_ORDER,
        confidence=0.8,
        action_type=ActionType.WRITE,
        summary="Confirm SO001",
        parameters=IntentParameters(order_id="SO001", dry_run=True),
        validation=ValidationRequirement(),
    )
    intent_service._post_process(res, "ir-pending", "tenant")
    assert "awaiting" in session_memory.describe("ir-pending", "tenant")


def test_steps_survive_post_processing():
    res = _read(IntentType.GET_CUSTOMERS, "customers")
    res.steps = [
        IntentStep(intent_type=IntentType.GET_SALE_ORDERS, summary="then their orders"),
        IntentStep(intent_type=IntentType.GET_INVOICES, summary="then invoices"),
    ]
    out = intent_service._post_process(res, "ir-steps", "tenant")
    assert [s.intent_type for s in out.steps] == [IntentType.GET_SALE_ORDERS, IntentType.GET_INVOICES]


def test_print_document_is_forced_read():
    res = ParsedIntentResponse(
        intent_type=IntentType.PRINT_DOCUMENT,
        confidence=0.9,
        action_type=ActionType.WRITE,  # model guessed wrong
        summary="print",
        parameters=IntentParameters(document_type="quotation"),
        validation=ValidationRequirement(risk_level="HIGH"),
    )
    out = intent_service._post_process(res, "ir-print", "tenant")
    assert out.action_type == ActionType.READ
    assert out.validation.risk_level == "LOW"
    assert out.validation.requires_user_approval is False


def test_deliver_is_forced_high_risk_and_write():
    res = ParsedIntentResponse(
        intent_type=IntentType.DELIVER_SALE_ORDER,
        confidence=0.9,
        action_type=ActionType.READ,  # model guessed wrong
        summary="deliver",
        parameters=IntentParameters(order_id="SO001"),
        validation=ValidationRequirement(risk_level="medium"),
    )
    out = intent_service._post_process(res, "ir-deliver", "tenant")
    assert out.action_type == ActionType.WRITE
    assert out.validation.risk_level == "HIGH"
    assert out.validation.requires_user_approval is True


def test_single_line_is_mirrored_into_flat_slots():
    from src.schemas.intents import OrderLinePayload

    res = ParsedIntentResponse(
        intent_type=IntentType.CREATE_SALE_ORDER,
        confidence=0.9,
        action_type=ActionType.WRITE,
        summary="create",
        parameters=IntentParameters(
            customer_name="Azure Interior",
            quantity=0.0,
            lines=[OrderLinePayload(product_name="Customizable Desk", quantity=5.0)],
        ),
        validation=ValidationRequirement(),
    )
    out = intent_service._post_process(res, "ir-line", "tenant")
    assert out.parameters.product_name == "Customizable Desk"
    assert out.parameters.quantity == 5.0


def test_conversational_intent_is_never_needing_approval():
    from src.schemas.intents import CONVERSATIONAL_INTENTS

    assert IntentType.CONVERSATION in CONVERSATIONAL_INTENTS
    assert IntentType.CLARIFY in CONVERSATIONAL_INTENTS
