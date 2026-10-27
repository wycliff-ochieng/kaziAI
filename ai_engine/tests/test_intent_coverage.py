import pytest
"""Full intent behavior cover through public parse_intent interface."""
from src.schemas.intents import ActionType, IntentType, ParseIntentRequest
from src.services.intent_service import intent_service


def parse(prompt):
    return intent_service.parse_intent(ParseIntentRequest(prompt=prompt))


@pytest.mark.skip(reason="LLM integration tests disabled locally")
def test_user_can_approve_pending_action_with_token():
    res = parse("Yes approve action token: CONFIRM_SO_501_123")
    assert res.intent_type == IntentType.APPROVE_ACTION
    assert res.action_type == ActionType.WRITE
    assert res.parameters.approval_token == "CONFIRM_SO_501_123"
    assert res.parameters.dry_run is False
    assert res.validation.requires_user_approval is False


@pytest.mark.skip(reason="LLM integration tests disabled locally")
def test_user_can_request_quotation_email():
    res = parse("Send quotation QUO101 via email")
    assert res.intent_type == IntentType.SEND_QUOTATION
    assert res.action_type == ActionType.WRITE
    assert res.parameters.quotation_id == "QUO101"
    assert res.validation.risk_level == "LOW"


@pytest.mark.skip(reason="LLM integration tests disabled locally")
def test_user_can_convert_quotation_to_order():
    res = parse("Convert quotation QUO101 into sale order")
    assert res.intent_type == IntentType.CONVERT_QUOTATION_TO_ORDER
    assert res.action_type == ActionType.WRITE
    assert res.parameters.quotation_id == "QUO101"
    assert res.validation.risk_level == "MEDIUM"


@pytest.mark.skip(reason="LLM integration tests disabled locally")
def test_user_can_cancel_sale_order_with_high_risk_warning():
    res = parse("Cancel sale order SO001")
    assert res.intent_type == IntentType.CANCEL_SALE_ORDER
    assert res.action_type == ActionType.WRITE
    assert res.parameters.order_id == "SO001"
    assert res.validation.risk_level == "HIGH"
    assert res.validation.requires_user_approval is True


@pytest.mark.skip(reason="LLM integration tests disabled locally")
def test_user_can_create_draft_invoice_for_order():
    res = parse("Generate draft invoice")
    assert res.intent_type == IntentType.CREATE_INVOICE
    assert res.action_type == ActionType.WRITE


@pytest.mark.skip(reason="LLM integration tests disabled locally")
def test_user_can_list_sale_orders_and_filter_draft():
    res = parse("List all draft sale orders")
    assert res.intent_type == IntentType.GET_SALE_ORDERS
    assert res.action_type == ActionType.READ
    assert res.parameters.state == "draft"
    assert res.validation.requires_user_approval is False


@pytest.mark.skip(reason="LLM integration tests disabled locally")
def test_user_can_view_sale_order_details():
    res = parse("View sale order SO001 details")
    assert res.intent_type == IntentType.GET_SALE_ORDER_DETAILS
    assert res.parameters.order_id == "SO001"


@pytest.mark.skip(reason="LLM integration tests disabled locally")
def test_user_can_search_products_catalog():
    res = parse("Find product Ergonomic Office Chair")
    assert res.intent_type == IntentType.GET_PRODUCTS
    assert res.validation.requires_user_approval is False


@pytest.mark.skip(reason="LLM integration tests disabled locally")
def test_user_can_list_overdue_invoices():
    res = parse("Show all overdue invoices")
    assert res.intent_type == IntentType.GET_OVERDUE_INVOICES
    assert res.validation.requires_user_approval is False


@pytest.mark.skip(reason="LLM integration tests disabled locally")
def test_user_can_list_regular_invoices():
    res = parse("Show all invoices")
    assert res.intent_type == IntentType.GET_INVOICES


@pytest.mark.skip(reason="LLM integration tests disabled locally")
def test_unknown_request_falls_back_without_approval():
    res = parse("What is the weather on Mars?")
    assert res.intent_type == IntentType.UNKNOWN
    assert res.validation.requires_user_approval is False


@pytest.mark.skip(reason="LLM integration tests disabled locally")
def test_live_execution_flag_disables_dry_run():
    res = parse("Create a sale order for Azure Interior for 5 units of Customizable Desk, real execution")
    assert res.intent_type == IntentType.CREATE_SALE_ORDER
    assert res.parameters.dry_run is False


@pytest.mark.skip(reason="LLM integration tests disabled locally")
def test_default_request_stays_in_dry_run():
    res = parse("Create a sale order for Azure Interior for 5 units of Customizable Desk")
    assert res.parameters.dry_run is True


@pytest.mark.skip(reason="LLM integration tests disabled locally")
def test_quotation_builds_order_line_from_product_and_quantity():
    res = parse("Draft a quotation for Azure Interior for 10 units of Ergonomic Office Chair")
    assert len(res.parameters.lines) == 1
    assert res.parameters.lines[0].quantity == 10.0
    assert "Chair" in (res.parameters.lines[0].product_name or "")
