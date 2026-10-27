"""Regression tests for slot-filling + strict entity extraction."""
from src.tools.backend_client import _sanitize_search
from src.graph.workflow import SYSTEM_PROMPT
from src.graph.state import AgentState
from langchain_core.messages import HumanMessage


def test_sanitize_strips_product_fragment_from_customer():
    assert _sanitize_search("Eslam product bl serum") == "eslam bl serum" or "eslam" in _sanitize_search("Eslam product bl serum")
    out = _sanitize_search("Eslam product bl serum")
    assert "product" not in out
    assert len(out.split()) <= 4


def test_sanitize_strips_have_products_as():
    out = _sanitize_search("have products as Aqualyx 80ml")
    assert "have" not in out
    assert "products" not in out
    assert "aqualyx" in out


def test_sanitize_keeps_bare_names():
    assert _sanitize_search("Akinyi Ojiambo") == "akinyi ojiambo"
    assert _sanitize_search("Eslam") == "eslam"
    assert _sanitize_search("") == ""


def test_system_prompt_has_slot_filling():
    assert "Missing slot" in SYSTEM_PROMPT
    assert "ask ONE question" in SYSTEM_PROMPT
    assert "Do not invent a customer" in SYSTEM_PROMPT
    assert "Write tools NEVER" in SYSTEM_PROMPT


def test_state_has_draft_slots():
    state = AgentState(
        tenant_id="t",
        user_id="u",
        channel="WEB",
        messages=[HumanMessage(content="Create order for Eslam")],
        intent_category="CRM",
        confidence_score=0.9,
        execution_plan=[],
        pending_tool_calls=[],
        tool_results=[],
        draft_customer_name="Eslam",
        draft_customer_id=None,
        draft_product_name=None,
        draft_product_id=None,
        draft_quantity=None,
        dry_run_preview=None,
        final_response_text=None,
        error=None,
    )
    assert state["draft_customer_name"] == "Eslam"
    assert state["draft_product_name"] is None
