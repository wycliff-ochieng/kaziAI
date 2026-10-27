import json

from fastapi import FastAPI
from fastapi.testclient import TestClient
from langchain_core.messages import AIMessage, HumanMessage, ToolMessage

from src.api.dependencies import get_agent
from src.api.routes import router


class FakeAgent:
    def __init__(self, messages):
        self.messages = messages
        self.calls = []

    async def ainvoke(self, state, config=None):
        self.calls.append({"state": state, "config": config})
        return {"messages": self.messages}


def _client(agent):
    app = FastAPI()
    app.include_router(router)
    app.dependency_overrides[get_agent] = lambda: agent
    return TestClient(app)


def _payload(**extra):
    return {"prompt": "question", "tenant_id": "t1", "conversation_id": "c1", **extra}


def test_process_read_returns_rows_and_plan():
    rows = [{"id": "201", "customer_name": "Azure Interior", "overdue": True}]
    agent = FakeAgent([
        HumanMessage(content="overdue invoices"),
        AIMessage(content="", tool_calls=[{"name": "get_invoices_tool", "args": {"overdue": True}, "id": "c1"}]),
        ToolMessage(content=json.dumps({"status": "success", "count": 1, "data": rows}), tool_call_id="c1", name="get_invoices_tool"),
        AIMessage(content="You have 1 overdue invoice."),
    ])
    with _client(agent) as client:
        resp = client.post("/api/v1/process", json=_payload())
    assert resp.status_code == 200
    body = resp.json()
    assert body["intent_type"] == "GET_INVOICES"
    assert body["action_type"] == "READ"
    assert body["requires_confirmation"] is False
    assert body["count"] == 1
    assert body["record_ids"] == ["201"]
    assert body["data"] == rows
    assert body["thread_id"] == "t1:c1"
    assert body["response"] == "You have 1 overdue invoice."
    assert agent.calls[0]["config"]["configurable"]["thread_id"] == "t1:c1"


def test_process_write_returns_preview_only_plan():
    preview = {
        "success": True,
        "message": "DryRun simulation",
        "dry_run": {"operation": "CREATE_SALE_ORDER", "confirmation_prompt": "Create the order?"},
    }
    agent = FakeAgent([
        HumanMessage(content="sell one desk to Azure and deliver"),
        AIMessage(content="", tool_calls=[{
            "name": "create_sale_order_tool",
            "args": {"customer_name": "Azure Interior", "product_name": "Desk", "quantity": 1, "deliver_immediately": True},
            "id": "w1",
        }]),
        ToolMessage(content=json.dumps(preview), tool_call_id="w1", name="create_sale_order_tool"),
        AIMessage(content="Here is the preview. Approve to create it."),
    ])
    with _client(agent) as client:
        resp = client.post("/api/v1/process", json=_payload())
    body = resp.json()
    assert body["requires_confirmation"] is True
    assert body["dry_run"] is not None
    assert body["intent_type"] == "CREATE_SALE_ORDER"
    steps = body["plan"]["steps"]
    assert [s["intent_type"] for s in steps] == ["CREATE_SALE_ORDER", "CONFIRM_SALE_ORDER", "DELIVER_SALE_ORDER"]
    assert all(s["parameters"]["dry_run"] is False for s in steps)
