import pytest
from src.graph.workflow import compiled_ai_engine
from src.graph.state import AgentState
from langchain_core.messages import HumanMessage

def test_workflow_compilation():
    """Verify that the AI engine state graph compiles securely."""
    assert compiled_ai_engine is not None
    assert "intent_router" in compiled_ai_engine.nodes
    assert "guardrails" in compiled_ai_engine.nodes

def test_state_schema():
    """Test typing definitions of the core graph state."""
    state = AgentState(
        tenant_id="test",
        user_id="user123",
        channel="WEB",
        messages=[HumanMessage(content="Hello AI!")],
        intent_category="FINANCE",
        confidence_score=0.99,
        execution_plan=[],
        pending_tool_calls=[],
        tool_results=[],
        dry_run_preview=None,
        final_response_text=None,
        error=None
    )
    assert state["tenant_id"] == "test"
    assert len(state["messages"]) == 1
