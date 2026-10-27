"""Tests for the deterministic entity-resolution policy.

Covers the stacked failure in the log: greedy slot extraction searched for a
whole sentence, and duplicates (multiple "Administrator" rows) would have been
resolved by silently picking the first hit.
"""
from src.services.entity_resolver import (
    ResolutionStatus,
    normalize_name,
    resolve,
    resolution_speech,
)

CUSTOMERS = [
    {"id": "866", "name": "Aarna Singh"},
    {"id": "3133", "name": "Eslam Mohamed"},
    {"id": "146", "name": "Administrator"},
    {"id": "176", "name": "Administrator"},
    {"id": "900", "name": "Acme Ltd"},
]


def test_exact_id_wins():
    res = resolve("3133", CUSTOMERS)
    assert res.status == ResolutionStatus.RESOLVED
    assert res.strategy == "exact_id"
    assert res.resolved_id == "3133"


def test_exact_name_case_insensitive():
    res = resolve("eslam mohamed", CUSTOMERS)
    assert res.status == ResolutionStatus.RESOLVED
    assert res.resolved_id == "3133"


def test_duplicate_names_are_ambiguous_not_guessed():
    res = resolve("Administrator", CUSTOMERS)
    assert res.status == ResolutionStatus.AMBIGUOUS
    assert res.best is None
    assert len(res.candidates) == 2
    assert "multiple" in resolution_speech(res, "customer").lower()


def test_normalization_strips_legal_suffix():
    assert normalize_name("Acme Limited") == "acme"
    res = resolve("Acme Ltd", CUSTOMERS)
    assert res.status == ResolutionStatus.RESOLVED
    assert res.resolved_id == "900"


def test_fuzzy_match_resolves_near_miss():
    res = resolve("Aarna Sing", CUSTOMERS)
    assert res.status == ResolutionStatus.RESOLVED
    assert res.resolved_id == "866"


def test_no_match_returns_not_found_with_suggestions():
    res = resolve("Zzzz Nobody", CUSTOMERS)
    assert res.status == ResolutionStatus.NOT_FOUND
    assert "couldn't find" in resolution_speech(res, "customer").lower()


def test_empty_query_is_rejected():
    res = resolve("   ", CUSTOMERS)
    assert res.status == ResolutionStatus.EMPTY_QUERY


def test_whole_sentence_is_not_a_name():
    # The resolver receives a pre-cleaned name; a sentence should not match.
    res = resolve("eslam mohamed and add bl serum and deliver the product", CUSTOMERS)
    assert res.status != ResolutionStatus.RESOLVED
