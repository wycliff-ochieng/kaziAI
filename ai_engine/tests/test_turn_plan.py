import json

from src.services.turn_plan import build_turn_plan


def _result(name, payload):
    return {"name": name, "content": json.dumps(payload)}


def test_read_turn_returns_data_and_frame():
    rows = [
        {"id": "9", "order_no": "S00009", "customer_id": "3133", "state": "sale"},
        {"id": "8", "order_no": "S00008", "customer_id": "3133", "state": "sale"},
    ]
    plan = build_turn_plan(
        [{"name": "get_sale_orders_tool", "args": {"customer_name": "Eslam"}}],
        [_result("get_sale_orders_tool", {"status": "success", "count": 2, "data": rows})],
    )
    assert plan["intent_type"] == "GET_SALE_ORDERS"
    assert plan["action_type"] == "READ"
    assert plan["requires_confirmation"] is False
    assert plan["count"] == 2
    assert plan["record_ids"] == ["9", "8"]
    assert plan["data"] == rows


def test_write_turn_is_preview_only_and_plan_is_typed():
    preview = {
        "success": True,
        "message": "DryRun simulation",
        "dry_run": {
            "operation": "CREATE_SALE_ORDER",
            "confirmation_prompt": "Create it?",
            "requires_confirmation": True,
        },
    }
    plan = build_turn_plan(
        [{
            "name": "create_sale_order_tool",
            "args": {
                "customer_name": "Eslam",
                "product_name": "Dp 4 filled Case",
                "quantity": 1,
                "deliver_immediately": True,
            },
        }],
        [_result("create_sale_order_tool", preview)],
    )
    assert plan["requires_confirmation"] is True
    assert plan["action_type"] == "WRITE"
    assert plan["intent_type"] == "CREATE_SALE_ORDER"
    assert plan["confirmation_prompt"] == "Create it?"
    assert [s["intent_type"] for s in plan["plan"]["steps"]] == [
        "CREATE_SALE_ORDER",
        "CONFIRM_SALE_ORDER",
        "DELIVER_SALE_ORDER",
    ]
    # Replay must be live; previews stay previews.
    assert plan["plan"]["steps"][0]["parameters"]["dry_run"] is False


def test_duplicate_write_calls_collapse():
    preview = {"success": True, "dry_run": {"confirmation_prompt": "ok"}}
    call = {"name": "confirm_sale_order_tool", "args": {"order_id": "9"}}
    plan = build_turn_plan([call, call], [_result("confirm_sale_order_tool", preview)])
    assert len(plan["plan"]["steps"]) == 1


def test_turn_messages_excludes_checkpoint_history():
    from langchain_core.messages import AIMessage, HumanMessage, ToolMessage
    from src.api.routes import _turn_messages

    history = [
        HumanMessage(content="old question"),
        AIMessage(content="", tool_calls=[{"name": "get_vendors_tool", "args": {}, "id": "old"}]),
        ToolMessage(content="{}", tool_call_id="old"),
        AIMessage(content="old answer"),
        HumanMessage(content="new question"),
        AIMessage(content="", tool_calls=[{"name": "get_customers_tool", "args": {}, "id": "new"}]),
        ToolMessage(content="{}", tool_call_id="new"),
        AIMessage(content="new answer"),
    ]

    turn = _turn_messages(history)
    assert isinstance(turn[0], HumanMessage)
    assert len(turn) == 4
    assert sum(len(getattr(m, "tool_calls", None) or []) for m in turn) == 1


def test_out_of_order_write_calls_are_sorted():
    preview = {"success": True, "dry_run": {"confirmation_prompt": "ok"}}
    plan = build_turn_plan(
        [
            {"name": "deliver_sale_order_tool", "args": {"order_id": "9"}},
            {"name": "confirm_sale_order_tool", "args": {"order_id": "9"}},
        ],
        [_result("deliver_sale_order_tool", preview), _result("confirm_sale_order_tool", preview)],
    )
    assert [s["intent_type"] for s in plan["plan"]["steps"]] == [
        "CONFIRM_SALE_ORDER",
        "DELIVER_SALE_ORDER",
    ]


def test_count_turn_returns_structured_count():
    plan = build_turn_plan(
        [{"name": "count_records_tool", "args": {"model_name": "invoices"}}],
        [_result("count_records_tool", {"status": "success", "count": 11, "metadata": {"entity": "invoices", "exact": True}})],
    )
    assert plan["intent_type"] == "COUNT_RECORDS"
    assert plan["count"] == 11
    assert plan["data"] == {"count": 11, "metadata": {"entity": "invoices", "exact": True}}
