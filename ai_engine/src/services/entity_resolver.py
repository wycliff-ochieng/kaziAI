"""Deterministic entity resolution.

Every tool used to implement its own ad-hoc search and silently picked the
first hit, which is how "eslam" collapsed the whole prompt tail into a
`res.partner` name. This module centralises one policy:

    1. exact ID
    2. exact name (case-insensitive)
    3. normalized name
    4. fuzzy match (rapidfuzz)
    5. multiple survivors -> contradictory -> ASK THE USER

It is deliberately pure: give it candidates (already fetched from the ERP) and
it returns a decision. Callers never "guess" between duplicates.
"""
from __future__ import annotations

import re
from dataclasses import dataclass, field
from enum import Enum
from typing import Any, Callable, Dict, Iterable, List, Optional, Sequence

try:  # rapidfuzz ships in pyproject; fall back cleanly if unavailable.
    from rapidfuzz import fuzz

    _HAS_RAPIDFUZZ = True
except Exception:  # pragma: no cover - dependency is declared, this is belt & braces
    _HAS_RAPIDFUZZ = False

_PUNCT_RE = re.compile(r"[^\w\s]")
_WS_RE = re.compile(r"\s+")

#: Suffixes that rarely disambiguate a business name.
_LEGAL_SUFFIXES = {"ltd", "limited", "inc", "llc", "plc", "co", "company", "group", "enterprises"}

DEFAULT_FUZZY_THRESHOLD = 88


class ResolutionStatus(str, Enum):
    RESOLVED = "resolved"
    AMBIGUOUS = "ambiguous"
    NOT_FOUND = "not_found"
    EMPTY_QUERY = "empty_query"


@dataclass
class ResolutionResult:
    status: ResolutionStatus
    best: Optional[Dict[str, Any]] = None
    candidates: List[Dict[str, Any]] = field(default_factory=list)
    strategy: str = ""
    message: str = ""

    @property
    def resolved_id(self) -> Optional[str]:
        if self.best is None:
            return None
        rid = self.best.get("id")
        return str(rid) if rid is not None else None


def normalize_name(value: str) -> str:
    text = str(value or "").casefold()
    text = _PUNCT_RE.sub(" ", text)
    tokens = [t for t in _WS_RE.split(text) if t]
    trimmed = [t for t in tokens if t not in _LEGAL_SUFFIXES] or tokens
    return " ".join(trimmed)


def _as_dict(item: Any) -> Dict[str, Any]:
    if isinstance(item, dict):
        return item
    if hasattr(item, "model_dump"):
        return item.model_dump()
    return vars(item)


def _score(query_norm: str, candidate_norm: str) -> float:
    if not query_norm or not candidate_norm:
        return 0.0
    if not _HAS_RAPIDFUZZ:
        return 100.0 if query_norm == candidate_norm else 0.0
    # Length-sensitive first: a whole sentence must not score 100 against a
    # 2-word name just because it contains those tokens.
    base = float(fuzz.ratio(query_norm, candidate_norm))
    if abs(len(query_norm.split()) - len(candidate_norm.split())) <= 2:
        base = max(base, float(fuzz.token_set_ratio(query_norm, candidate_norm)))
    return base


def resolve(
    query: str,
    candidates: Iterable[Any],
    *,
    id_key: str = "id",
    name_key: str = "name",
    alt_keys: Sequence[str] = (),
    fuzzy_threshold: float = DEFAULT_FUZZY_THRESHOLD,
) -> ResolutionResult:
    """Resolve ``query`` against already-fetched ``candidates``."""
    raw_query = str(query or "").strip()
    rows = [_as_dict(c) for c in candidates or []]
    if not raw_query:
        return ResolutionResult(status=ResolutionStatus.EMPTY_QUERY, message="No search term provided.")

    # 1. Exact ID
    for row in rows:
        if str(row.get(id_key, "")) == raw_query:
            return ResolutionResult(ResolutionStatus.RESOLVED, row, [row], "exact_id")

    query_norm = normalize_name(raw_query)
    query_lower = raw_query.casefold()

    def keys(row: Dict[str, Any]) -> List[str]:
        vals = [str(row.get(name_key, ""))]
        vals.extend(str(row.get(k, "")) for k in alt_keys)
        return [v for v in vals if v]

    # 2. Exact name / alt key
    exact = [r for r in rows if any(v.casefold() == query_lower for v in keys(r))]
    if len(exact) == 1:
        return ResolutionResult(ResolutionStatus.RESOLVED, exact[0], exact, "exact_name")
    if len(exact) > 1:
        return ResolutionResult(
            ResolutionStatus.AMBIGUOUS,
            None,
            exact,
            "exact_name",
            f"{len(exact)} records share the exact name '{raw_query}'.",
        )

    # 3. Normalized name
    normalized = [r for r in rows if any(normalize_name(v) == query_norm for v in keys(r))]
    if len(normalized) == 1:
        return ResolutionResult(ResolutionStatus.RESOLVED, normalized[0], normalized, "normalized_name")
    if len(normalized) > 1:
        return ResolutionResult(
            ResolutionStatus.AMBIGUOUS,
            None,
            normalized,
            "normalized_name",
            f"{len(normalized)} records match '{raw_query}' after normalization.",
        )

    # 4. Fuzzy match
    scored = []
    for row in rows:
        best = max((_score(query_norm, normalize_name(v)) for v in keys(row)), default=0.0)
        scored.append((best, row))
    scored.sort(key=lambda pair: pair[0], reverse=True)
    strong = [r for score, r in scored if score >= fuzzy_threshold]

    if len(strong) == 1:
        return ResolutionResult(ResolutionStatus.RESOLVED, strong[0], strong, "fuzzy")
    if len(strong) > 1:
        return ResolutionResult(
            ResolutionStatus.AMBIGUOUS,
            None,
            strong,
            "fuzzy",
            f"{len(strong)} records are a close match for '{raw_query}'.",
        )

    return ResolutionResult(
        ResolutionStatus.NOT_FOUND,
        None,
        [r for _, r in scored[:5]],
        "none",
        f"No record matched '{raw_query}'.",
    )


def resolution_speech(result: ResolutionResult, label: str = "record") -> str:
    """Deterministic clarification/error text for the user."""
    if result.status == ResolutionStatus.RESOLVED:
        name = result.best.get("name") if result.best else ""
        return f"Resolved {label} to {name or result.resolved_id}."
    if result.status == ResolutionStatus.AMBIGUOUS:
        options = ", ".join(str(r.get("name") or r.get("id")) for r in result.candidates[:5])
        return f"I found multiple {label}s matching your request ({options}). Which one did you mean?"
    if result.status == ResolutionStatus.NOT_FOUND:
        return f"I couldn't find any {label} matching that name."
    return result.message or f"I need a clearer {label} name."
