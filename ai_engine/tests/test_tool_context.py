from src.tools.backend_client import (
    backend_client,
    reset_tool_context,
    set_tool_context,
)


def test_tool_context_routes_tenant_and_erp_type():
    token = set_tool_context("t1", "mock")
    try:
        assert backend_client._headers()["X-Tenant-ID"] == "t1"
        assert backend_client._params({"limit": 5})["erp_type"] == "MOCK"
    finally:
        reset_tool_context(token)

    assert "X-Tenant-ID" not in backend_client._headers()
    assert "erp_type" not in backend_client._params({"limit": 5})
