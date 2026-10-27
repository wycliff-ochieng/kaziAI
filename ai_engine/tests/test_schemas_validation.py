"""Schema defaults and validation through public Pydantic models."""
import pytest
from pydantic import ValidationError

from src.schemas.intents import (
    ActionType,
    IntentParameters,
    IntentType,
    ParsedIntentResponse,
    ParseIntentRequest,
    ValidationRequirement,
)


def test_parse_request_applies_tenant_and_dry_run_defaults():
    req = ParseIntentRequest(prompt="Show me all active customers")
    assert req.tenant_id == "default_tenant"
    assert req.context == {}


def test_parse_request_rejects_empty_prompt_payload():
    with pytest.raises(ValidationError):
        ParseIntentRequest(prompt=None)  # type: ignore


def test_intent_parameters_default_to_safe_dry_run():
    params = IntentParameters()
    assert params.dry_run is True
    assert params.quantity == 1.0
    assert params.format == "pdf"
    assert params.lines == []


def test_validation_defaults_require_approval_at_medium_risk():
    validation = ValidationRequirement()
    assert validation.requires_user_approval is True
    assert validation.risk_level == "MEDIUM"


def test_parsed_response_round_trips_through_public_schema():
    res = ParsedIntentResponse(
        intent_type=IntentType.GET_CUSTOMERS,
        confidence=0.9,
        action_type=ActionType.READ,
        summary="Fetch customers list",
        parameters=IntentParameters(search="azure"),
        validation=ValidationRequirement(requires_user_approval=False),
    )
    assert res.intent_type == IntentType.GET_CUSTOMERS
    assert res.parameters.search == "azure"
    assert res.validation.requires_user_approval is False
