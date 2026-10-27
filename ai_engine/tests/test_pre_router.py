"""Regression tests for the deterministic pre-router.

Every case here comes straight from the live log: chit-chat was being routed
into ERP execution (and, on the on-demand tier, crashing with a 429).
"""
from src.schemas.intents import IntentType, ParseIntentRequest
from src.services.pre_router import classify_conversation, pre_route
from src.services.intent_service import intent_service


def test_greeting_is_conversation():
    res = pre_route("hello")
    assert res is not None
    assert res.intent_type == IntentType.CONVERSATION
    assert res.validation.requires_user_approval is False


def test_short_affirm_is_affirm_intent():
    for text in ["okay", "ok", "yes", "go ahead", "confirmed"]:
        res = pre_route(text)
        assert res is not None and res.intent_type == IntentType.AFFIRM, text


def test_short_reject_is_reject_intent():
    for text in ["no", "cancel that", "stop", "never mind"]:
        res = pre_route(text)
        assert res is not None and res.intent_type == IntentType.REJECT, text


def test_doubt_phrase_is_conversation_not_confirm():
    # "are you sure" previously became CONFIRM_SALE_ORDER -> "sales order ID required".
    res = pre_route("are you sure")
    assert res is not None
    assert res.intent_type == IntentType.CONVERSATION
    assert classify_conversation("really?") == "doubt"


def test_correction_phrase_is_conversation():
    for text in ["why are you still guessing", "stop guessing", "those are sale orders not purchase orders"]:
        res = pre_route(text)
        assert res is not None, text
        assert res.intent_type == IntentType.CONVERSATION, text


def test_meta_questions_are_conversation():
    for text in ["what can you do?", "who are you", "help"]:
        res = pre_route(text)
        assert res is not None and res.intent_type == IntentType.CONVERSATION, text


def test_thanks_and_farewell_are_conversation():
    assert pre_route("thanks").intent_type == IntentType.CONVERSATION
    assert pre_route("goodbye").intent_type == IntentType.CONVERSATION


def test_real_erp_requests_fall_through_to_llm():
    for text in [
        "show vendors",
        "show first 10 customers",
        "show purchases",
        "show customers with more than one sale order",
        "list all invoices",
        "hi, show my first 10 customers",
    ]:
        assert pre_route(text) is None, text


def test_empty_prompt_is_conversation():
    assert pre_route("").intent_type == IntentType.CONVERSATION


def test_parse_intent_short_circuits_chit_chat_without_llm():
    # No network/API key needed: the pre-router must answer before any LLM call.
    res = intent_service.parse_intent(ParseIntentRequest(prompt="hello"))
    assert res.intent_type == IntentType.CONVERSATION
    assert res.summary
