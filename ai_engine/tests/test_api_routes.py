"""Tracer bullet: AI Engine public HTTP surface.

Verifies behavior through FastAPI routes (public interface), not internals.
"""
import os

import pytest
from fastapi.testclient import TestClient

from src.main import app


client = TestClient(app)


def test_health_check_reports_healthy_service():
    res = client.get("/health")
    assert res.status_code == 200
    body = res.json()
    assert body["status"] == "healthy"
    assert body["service"] == "ai_engine"


@pytest.mark.integration
@pytest.mark.skipif(not os.getenv("RUN_LLM_TESTS"), reason="requires a live LLM provider")
def test_parse_intent_endpoint_parses_quotation_through_public_api():
    res = client.post(
        "/api/v1/parse-intent",
        json={"prompt": "Draft a quotation for Azure Interior for 10 units of Ergonomic Office Chair"},
    )
    assert res.status_code == 200
    body = res.json()
    assert body["intent_type"] == "CREATE_QUOTATION"
    assert body["action_type"] == "WRITE"
    assert body["parameters"]["customer_name"] == "Azure Interior"


@pytest.mark.integration
@pytest.mark.skipif(not os.getenv("RUN_LLM_TESTS"), reason="requires a live LLM provider")
def test_chat_endpoint_returns_parsed_intent_envelope():
    res = client.post(
        "/api/v1/chat",
        json={"prompt": "Show me all active customers"},
    )
    assert res.status_code == 200
    body = res.json()
    assert body["status"] == "success"
    assert body["parsed_intent"]["intent_type"] == "GET_CUSTOMERS"
