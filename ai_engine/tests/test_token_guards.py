"""Regression tests for Groq 7k-token budget guards (413 fix)."""
from fastapi.testclient import TestClient
from langchain_core.messages import HumanMessage, AIMessage

from src.graph.workflow import (
    _trim_messages,
    _truncate_text,
    MAX_HISTORY_MESSAGES,
    MAX_TOOL_CHARS,
    compiled_ai_engine,
)
from src.tools.backend_client import _cap_payload, MAX_ROWS_PER_TOOL_RESULT
from src.main import app
from src.api.dependencies import get_agent


def _msgs(n):
    return [HumanMessage(content=f"m{i}") for i in range(n)]


def test_trim_keeps_sliding_window():
    msgs = _msgs(MAX_HISTORY_MESSAGES + 5)
    trimmed = _trim_messages(msgs)
    assert len(trimmed) == MAX_HISTORY_MESSAGES
    assert trimmed[0].content == f"m{5}"
    assert trimmed[-1].content == f"m{MAX_HISTORY_MESSAGES + 4}"


def test_trim_short_history_untouched():
    msgs = _msgs(3)
    assert _trim_messages(msgs) == msgs
    assert _trim_messages([]) == []


def test_truncate_long_tool_output():
    long_text = "x" * (MAX_TOOL_CHARS + 500)
    out = _truncate_text(long_text)
    assert len(out) < len(long_text)
    assert "truncated" in out
    assert _truncate_text("short") == "short"


def test_cap_payload_dict_list():
    payload = {"data": [{"id": i} for i in range(MAX_ROWS_PER_TOOL_RESULT + 10)]}
    capped = _cap_payload(payload)
    assert len(capped["data"]) == MAX_ROWS_PER_TOOL_RESULT
    assert "data_truncated" in capped


def test_cap_payload_raw_list():
    payload = [{"id": i} for i in range(MAX_ROWS_PER_TOOL_RESULT + 5)]
    assert len(_cap_payload(payload)) == MAX_ROWS_PER_TOOL_RESULT
    assert _cap_payload({"ok": True}) == {"ok": True}


def test_workflow_has_token_guard_nodes():
    assert "agent" in compiled_ai_engine.nodes
    assert "tools" in compiled_ai_engine.nodes


def _client_with_fake_agent(fake):
    app.dependency_overrides[get_agent] = lambda: fake
    try:
        yield_test = TestClient(app)
        return yield_test
    finally:
        pass


def test_reset_endpoint_returns_hint():
    class FakeAgent:
        pass

    app.dependency_overrides[get_agent] = lambda: FakeAgent()
    try:
        client = TestClient(app)
        res = client.post("/api/v1/reset", json={"prompt": "hi", "thread_id": "t1"})
        assert res.status_code == 200
        body = res.json()
        assert body["status"] == "success"
        assert body["thread_id"] == "t1"
    finally:
        app.dependency_overrides.clear()


def test_process_truncates_long_prompt():
    seen = {}

    class FakeAgent:
        async def ainvoke(self, payload, config=None):
            seen["prompt"] = payload["messages"][0].content
            seen["thread"] = (config or {}).get("configurable", {}).get("thread_id")
            from langchain_core.messages import AIMessage as AIM

            return {"messages": [AIM(content="ok")]}

    app.dependency_overrides[get_agent] = lambda: FakeAgent()
    try:
        client = TestClient(app)
        long_prompt = "y" * 3000
        res = client.post("/api/v1/process", json={"prompt": long_prompt, "thread_id": "t2"})
        assert res.status_code == 200
        assert len(seen["prompt"]) == 2000
        assert seen["thread"] == "t2"
    finally:
        app.dependency_overrides.clear()


def test_process_maps_413_to_429():
    class BoomAgent:
        async def ainvoke(self, payload, config=None):
            raise RuntimeError("Error code: 413 - {'error': {'code': 'rate_limit_exceeded'}}")

    app.dependency_overrides[get_agent] = lambda: BoomAgent()
    try:
        client = TestClient(app)
        res = client.post("/api/v1/process", json={"prompt": "list vendors", "thread_id": "t3"})
        assert res.status_code == 429
        assert "fresh conversation_id" in res.json()["detail"]
    finally:
        app.dependency_overrides.clear()
