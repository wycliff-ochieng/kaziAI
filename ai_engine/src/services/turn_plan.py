"""Turn assembly for the agent loop.

The ReAct agent executes tools, but the gateway (Go) needs a typed result:
what was read (for UI), and what is being proposed as a write (for approval).
This module is the seam between "the model called tools" and "the system
executes". It never calls a model and never executes anything.

Contract:
    build_turn_plan(tool_calls, tool_results) -> dict

The returned dict is serialized by the /process route and consumed by Go.
"""
from __future__ import annotations

import json
from typing import Any, Dict, Iterable, List, Optional, Tuple

#: tool name -> (canonical intent, logical resource)
READ_TOOLS: Dict[str, Tuple[str, str]] = {
    "get_customers_tool": ("GET_CUSTOMERS", "customers"),
    "get_products_tool": ("GET_PRODUCTS", "products"),
    "get_sale_orders_tool": ("GET_SALE_ORDERS", "orders"),
    "get_invoices_tool": ("GET_INVOICES", "invoices"),
    "get_vendors_tool": ("GET_VENDORS", "vendors"),
    "get_purchase_orders_tool": ("GET_PURCHASE_ORDERS", "purchase_orders"),
    "get_purchase_order_by_id_tool": ("GET_PURCHASE_ORDER_DETAILS", "purchase_orders"),
    "count_records_tool": ("COUNT_RECORDS", ""),
}

#: tool name -> canonical write intent
WRITE_TOOLS: Dict[str, str] = {
    "create_sale_order_tool": "CREATE_SALE_ORDER",
    "confirm_sale_order_tool": "CONFIRM_SALE_ORDER",
    "deliver_sale_order_tool": "DELIVER_SALE_ORDER",
    "create_invoice_tool": "CREATE_INVOICE",
    "post_invoice_tool": "POST_INVOICE",
    "create_rfq_tool": "CREATE_RFQ",
    "confirm_purchase_order_tool": "CONFIRM_PURCHASE_ORDER",
    "receive_purchase_order_tool": "RECEIVE_PURCHASE_ORDER",
    "create_vendor_bill_tool": "CREATE_VENDOR_BILL",
    "post_vendor_bill_tool": "POST_VENDOR_BILL",
    "cancel_purchase_order_tool": "CANCEL_PURCHASE_ORDER",
}

#: Order matters: it becomes the execution order of the approved plan.
_WRITE_ORDER = [
    "CREATE_SALE_ORDER",
    "CONFIRM_SALE_ORDER",
    "DELIVER_SALE_ORDER",
    "CREATE_INVOICE",
    "POST_INVOICE",
    "CREATE_RFQ",
    "CONFIRM_PURCHASE_ORDER",
    "RECEIVE_PURCHASE_ORDER",
    "CREATE_VENDOR_BILL",
    "POST_VENDOR_BILL",
    "CANCEL_PURCHASE_ORDER",
]

CONVERSATION_PROMPT = "How can I help with your Odoo data?"


def _as_json(value: Any) -> Any:
    if isinstance(value, (dict, list)):
        return value
    if isinstance(value, str):
        text = value.strip()
        if not text:
            return None
        try:
            return json.loads(text)
        except (TypeError, ValueError):
            return None
    return None


def _payload(content: Any) -> Any:
    """Tool output is usually a JSON string, sometimes already a dict."""
    parsed = _as_json(content)
    if isinstance(parsed, dict):
        return parsed
    return None


def _data_from_payload(payload: Any) -> Any:
    if not isinstance(payload, dict):
        return None
    if payload.get("error"):
        return None
    data = payload.get("data")
    if data is not None:
        return data
    for key in ("customers", "vendors", "products", "orders", "invoices", "items", "results"):
        if isinstance(payload.get(key), list):
            return payload[key]
    return None


def _rows(data: Any) -> List[Dict[str, Any]]:
    if isinstance(data, list):
        return [r for r in data if isinstance(r, dict)]
    if isinstance(data, dict):
        return [data]
    return []


def _lines(args: Dict[str, Any]) -> List[Dict[str, Any]]:
    raw = args.get("lines")
    if isinstance(raw, list) and raw:
        return [l for l in raw if isinstance(l, dict)]
    if args.get("product_id") or args.get("product_name"):
        return [
            {
                "product_id": args.get("product_id") or "",
                "product_name": args.get("product_name") or "",
                "quantity": args.get("quantity") or 1.0,
                "unit_price": args.get("unit_price") or 0.0,
            }
        ]
    return []


def _write_step(name: str, args: Dict[str, Any]) -> Optional[Dict[str, Any]]:
    intent = WRITE_TOOLS.get(name)
    if not intent:
        return None

    params: Dict[str, Any] = {"dry_run": False}
    if intent in ("CREATE_SALE_ORDER", "CREATE_QUOTATION"):
        params.update({
            "customer_id": args.get("customer_id") or "",
            "customer_name": args.get("customer_name") or "",
            "lines": _lines(args),
            "product_name": args.get("product_name") or "",
            "product_id": args.get("product_id") or "",
            "quantity": args.get("quantity") or 1.0,
            "unit_price": args.get("unit_price") or 0.0,
            "note": args.get("note") or "",
        })
    elif intent == "CREATE_RFQ":
        params.update({
            "vendor_id": args.get("vendor_id") or "",
            "vendor_name": args.get("vendor_name") or "",
            "lines": _lines(args),
            "product_name": args.get("product_name") or "",
            "product_id": args.get("product_id") or "",
            "quantity": args.get("quantity") or 1.0,
            "unit_price": args.get("unit_price") or 0.0,
            "note": args.get("note") or "",
        })
    elif intent in ("CREATE_VENDOR_BILL", "RECEIVE_PURCHASE_ORDER",
                    "CONFIRM_PURCHASE_ORDER", "CANCEL_PURCHASE_ORDER"):
        params["order_id"] = args.get("order_id") or args.get("bill_id") or ""
    elif intent == "POST_VENDOR_BILL":
        params["bill_id"] = args.get("bill_id") or args.get("order_id") or ""
    elif intent in ("CONFIRM_SALE_ORDER", "DELIVER_SALE_ORDER", "CREATE_INVOICE"):
        params["order_id"] = args.get("order_id") or ""
    elif intent == "POST_INVOICE":
        params["invoice_id"] = args.get("invoice_id") or ""

    return {"intent_type": intent, "summary": "", "parameters": params}


def build_turn_plan(
    tool_calls: Iterable[Dict[str, Any]],
    tool_results: Iterable[Dict[str, Any]],
) -> Dict[str, Any]:
    """Assemble one turn's read data and write plan from executed tool calls."""
    calls = [c for c in (tool_calls or []) if isinstance(c, dict) and c.get("name")]
    results = [r for r in (tool_results or []) if isinstance(r, dict) and r.get("name")]

    # Last successful data payload wins: that is what the user is looking at.
    data: Any = None
    last_payload: Any = None
    for result in results:
        payload = _payload(result.get("content"))
        if payload is None:
            continue
        last_payload = payload
        candidate = _data_from_payload(payload)
        if candidate is not None:
            data = candidate

    # Write steps, then expand "create and deliver" into an ordered chain.
    steps: List[Dict[str, Any]] = []
    for call in calls:
        step = _write_step(str(call.get("name")), call.get("args") or {})
        if step is None:
            continue
        steps.append(step)
        if step["intent_type"] == "CREATE_SALE_ORDER":
            args = call.get("args") or {}
            if args.get("deliver_immediately"):
                steps.append({"intent_type": "CONFIRM_SALE_ORDER", "summary": "",
                              "parameters": {"order_id": "", "dry_run": False}})
                steps.append({"intent_type": "DELIVER_SALE_ORDER", "summary": "",
                              "parameters": {"order_id": "", "dry_run": False}})

    # De-duplicate identical steps while preserving order (model retries happen).
    seen = set()
    deduped: List[Dict[str, Any]] = []
    for step in steps:
        key = (step["intent_type"], json.dumps(step["parameters"], sort_keys=True, default=str))
        if key in seen:
            continue
        seen.add(key)
        deduped.append(step)

    # Model-emitted calls can arrive out of order; execution must not.
    deduped.sort(key=lambda s: _WRITE_ORDER.index(s["intent_type"])
                 if s["intent_type"] in _WRITE_ORDER else len(_WRITE_ORDER))

    primary_write = deduped[0]["intent_type"] if deduped else ""
    primary_read, read_resource = "", ""
    for call in calls:
        mapping = READ_TOOLS.get(str(call.get("name")))
        if mapping and not primary_read:
            primary_read, read_resource = mapping
            break
    if not primary_read:
        # A tool we do not map still executed; infer a generic read.
        for result in results:
            if result.get("name") not in WRITE_TOOLS:
                mapping = READ_TOOLS.get(str(result.get("name")))
                if mapping:
                    primary_read, read_resource = mapping
                    break

    if data is None and primary_read == "COUNT_RECORDS" and isinstance(last_payload, dict):
        data = {"count": int(last_payload.get("count") or 0)}
        if isinstance(last_payload.get("metadata"), dict):
            data["metadata"] = last_payload["metadata"]

    dry_run: Dict[str, Any] = {}
    for result in results:
        payload = _payload(result.get("content"))
        if not isinstance(payload, dict):
            continue
        candidate = payload.get("dry_run")
        if isinstance(candidate, dict):
            dry_run = candidate

    confirmation_prompt = str(
        dry_run.get("confirmation_prompt")
        or (last_payload or {}).get("confirmation_prompt")
        or (last_payload or {}).get("message")
        or ""
    )
    if deduped and not confirmation_prompt:
        confirmation_prompt = "Do you want to proceed with this action?"

    rows = _rows(data)
    if isinstance(last_payload, dict) and "count" in last_payload:
        count = int(last_payload.get("count") or 0)
    else:
        count = len(rows)

    intent_type = primary_write or primary_read or "CONVERSATION"
    action_type = "WRITE" if deduped else ("READ" if primary_read else "CONVERSATION")

    return {
        "intent_type": intent_type,
        "action_type": action_type,
        "resource": read_resource,
        "requires_confirmation": bool(deduped),
        "confirmation_prompt": confirmation_prompt,
        "plan": {"steps": deduped} if deduped else None,
        "dry_run": dry_run or None,
        "tool_calls": [{"name": str(c.get("name")), "args": c.get("args") or {}} for c in calls],
        "data": data,
        "count": count,
        "record_ids": [str(r.get("id")) for r in rows if r.get("id") is not None][:50],
    }
