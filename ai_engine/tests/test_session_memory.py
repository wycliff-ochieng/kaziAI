"""Tests for the compact session state-frame used for multi-turn chaining."""
from src.services.session_memory import SessionMemory


def test_empty_frame_describes_as_no_history():
    mem = SessionMemory(ttl_seconds=60)
    assert "No prior turns" in mem.describe("t1", "tenant")


def test_record_query_populates_frame():
    mem = SessionMemory(ttl_seconds=60)
    mem.record_query(
        thread_id="t1",
        tenant_id="tenant",
        intent_type="GET_SALE_ORDERS",
        resource="orders",
        record_ids=["101", "102", "103"],
        result_count=3,
        filters=[{"field": "state", "operator": "=", "value": "sale"}],
        domain=[["state", "=", "sale"]],
    )
    desc = mem.describe("t1", "tenant")
    assert "GET_SALE_ORDERS" in desc
    assert "101" in desc
    assert "state = sale" in desc


def test_threads_are_isolated():
    mem = SessionMemory(ttl_seconds=60)
    mem.record_query("a", "tenant", "GET_CUSTOMERS", resource="customers", result_count=5)
    assert "GET_CUSTOMERS" not in mem.describe("b", "tenant")
    assert "GET_CUSTOMERS" in mem.describe("a", "tenant")


def test_ttl_expiry_drops_stale_frame():
    mem = SessionMemory(ttl_seconds=0)
    mem.record_query("t", "tenant", "GET_CUSTOMERS", result_count=5)
    # TTL 0 means anything already recorded is stale on next access.
    mem._store["tenant:t"].updated_at -= 1
    assert "No prior turns" in mem.describe("t", "tenant")


def test_pending_action_is_surfaced_and_clearable():
    mem = SessionMemory(ttl_seconds=60)
    mem.set_pending("t", "tenant", "CONFIRM_SALE_ORDER", "confirm SO001")
    assert "awaiting" in mem.describe("t", "tenant")
    mem.clear_pending("t", "tenant")
    assert "awaiting" not in mem.describe("t", "tenant")


def test_clear_removes_frame():
    mem = SessionMemory(ttl_seconds=60)
    mem.record_query("t", "tenant", "GET_CUSTOMERS", result_count=1)
    mem.clear("t", "tenant")
    assert "No prior turns" in mem.describe("t", "tenant")


def test_record_ids_capped():
    mem = SessionMemory(ttl_seconds=60)
    mem.record_query("t", "tenant", "GET_CUSTOMERS", record_ids=[str(i) for i in range(200)], limit=10)
    assert len(mem.get("t", "tenant").last_record_ids) == 10
