"""Deterministic conversational pre-router.

The LLM is expensive, rate-limited, and *wrong* for chit-chat: the live log
shows "hello" being classified as GET_CUSTOMER_DETAILS and, on the on-demand
Groq tier, failing outright with a 429. Anything recognisably conversational is
resolved here, before the LLM is ever called.

The router is intentionally conservative: a message that merely *contains* a
greeting but also asks for ERP data ("hi, show my last 10 customers") is left
for the semantic model.
"""
from __future__ import annotations

import re
from typing import Optional

from src.schemas.intents import (
    ActionType,
    IntentParameters,
    IntentType,
    ParsedIntentResponse,
    ValidationRequirement,
)

_GREETING_RE = re.compile(
    r"^(hi|hii+|hey+|hello+|helo|yo|howdy|good\s+(morning|afternoon|evening|day)|greetings|sup)\b",
    re.IGNORECASE,
)
_THANKS_RE = re.compile(r"^(thanks|thank\s+you|thankyou|thx|much\s+appreciated|cheers)\b", re.IGNORECASE)
_FAREWELL_RE = re.compile(r"^(bye|goodbye|see\s+you|good\s+night|cya|later)\b", re.IGNORECASE)

#: Short "yes/no" replies are mirrored by the Go gateway; kept here so the AI
#: engine agrees when called directly (e.g. tests, HTTP clients).
_AFFIRM_RE = re.compile(
    r"^(ok(ay)?|yes(\s+please)?|confirmed?|go(\s+ahead|\s+on)?|proceed|do\s+it|approved?|"
    r"looks\s+good|yep|yeah|sure|please\s+proceed)$",
    re.IGNORECASE,
)
_REJECT_RE = re.compile(
    r"^(no(pe)?|reject(ed)?|cancel(\s+that|\s+it)?|stop|don't|do\s+not|discard|never\s+mind|not\s+now)$",
    re.IGNORECASE,
)

#: Meta / capability questions.
_META_PATTERNS = [
    re.compile(r"\bwhat\s+(can|do)\s+you\s+(do|help)\b", re.IGNORECASE),
    re.compile(r"\bwhat\s+are\s+you\b", re.IGNORECASE),
    re.compile(r"\bwho\s+are\s+you\b", re.IGNORECASE),
    re.compile(r"\byour\s+(capabilities|abilities|features)\b", re.IGNORECASE),
    re.compile(r"^(help|help\s+me|how\s+do\s+i\s+use\s+(you|this))\b", re.IGNORECASE),
]

#: Acknowledged mistakes / complaints about guessing. Purely conversational:
#: the correct response is to re-anchor, never to fire another ERP mutation.
_CORRECTION_PATTERNS = [
    # Any talk of "guessing" is a complaint about the assistant's behaviour.
    re.compile(r"\bguess(ing|ed)?\b", re.IGNORECASE),
    re.compile(r"\b(you\s+are|you'?re|that'?s|that\s+is|this\s+is)\s+(wrong|incorrect|not\s+right)\b", re.IGNORECASE),
    re.compile(r"\bnot\s+what\s+i\s+(asked|meant)\b", re.IGNORECASE),
    # "those are sale orders not purchase orders" — a correction of a prior result.
    re.compile(r"^(those|these|that|this|they)\s+(are|is|were|was)\b.*\bnot\b", re.IGNORECASE),
    re.compile(r"\bdon'?t\s+guess\b", re.IGNORECASE),
]

_DOUBT_PATTERNS = [
    re.compile(r"^are\s+you\s+sure\b", re.IGNORECASE),
    re.compile(r"^really\?*$", re.IGNORECASE),
    re.compile(r"^you\s+sure\?*$", re.IGNORECASE),
]

_SHORT_WORDS = 6


def _normalize(text: str) -> str:
    return " ".join(text.strip().lower().rstrip(".!? ").split())


def _word_count(text: str) -> int:
    return len(text.split())


def _is_pure(pattern: re.Pattern, normalized: str) -> bool:
    """True only when the whole message is the matched phrase (plus filler)."""
    match = pattern.match(normalized)
    if not match:
        return False
    rest = normalized[match.end():].strip(" ,.!?")
    return rest in ("", "there", "all", "everyone")


def _conversation(summary: str, confidence: float = 1.0) -> ParsedIntentResponse:
    return ParsedIntentResponse(
        intent_type=IntentType.CONVERSATION,
        confidence=confidence,
        action_type=ActionType.CONVERSATION,
        summary=summary,
        parameters=IntentParameters(dry_run=False),
        validation=ValidationRequirement(
            requires_user_approval=False,
            risk_level="LOW",
            approval_prompt="",
            impact_summary="",
        ),
    )


def _classify_kind(text: str) -> Optional[str]:
    lowered = text.strip()
    normalized = _normalize(lowered)
    if not normalized:
        return "empty"

    words = _word_count(normalized)

    # Short replies only: never let "yes, confirm the whole order" match.
    if words <= _SHORT_WORDS and _AFFIRM_RE.match(normalized):
        return "affirm"
    if words <= _SHORT_WORDS and _REJECT_RE.match(normalized):
        return "reject"

    # Whole-message greetings/thanks/farewells only (no embedded request):
    # "hi, show my first 10 customers" must NOT be swallowed here.
    if _is_pure(_GREETING_RE, normalized):
        return "greeting"
    if _is_pure(_THANKS_RE, normalized):
        return "thanks"
    if _is_pure(_FAREWELL_RE, normalized):
        return "farewell"

    for pat in _META_PATTERNS:
        if pat.search(lowered):
            return "meta"
    for pat in _CORRECTION_PATTERNS:
        if pat.search(lowered):
            return "correction"
    for pat in _DOUBT_PATTERNS:
        if pat.search(lowered):
            return "doubt"

    return None


_CANNED: dict[str, str] = {
    "greeting": (
        "Hello! I'm your Odoo operations assistant. I can look up customers, vendors, "
        "products, sales orders, purchase orders and invoices, and prepare changes for "
        "your approval. Try \"show my first 10 customers\" or \"show unpaid invoices\"."
    ),
    "thanks": "You're welcome! Anything else you'd like to look up in Odoo?",
    "farewell": "Goodbye! Come back any time you need something from Odoo.",
    "meta": (
        "I'm an AI assistant wired into your Odoo ERP. I can list and search master data "
        "(customers, vendors, products), read sales and purchase documents, filter by state "
        "or relationships. Writes are shown as previews first; nothing runs until you "
        "approve it. Tell me what you need in plain language."
    ),
    "correction": (
        "You're right to push back — I won't guess. Tell me precisely which records you want "
        "(for example \"show sale orders\" vs \"show purchase orders\"), or name the customer "
        "and I'll re-query Odoo and show you exactly what it returns."
    ),
    "doubt": (
        "I only report what Odoo returns — no invented records. If a result looks wrong, tell "
        "me the exact query (e.g. \"show customers with more than one sale order\") and I'll "
        "run it and show the underlying data."
    ),
    "affirm": "There's no pending action for me to confirm right now.",
    "reject": "Okay, nothing was executed.",
    "empty": "I didn't catch that — what would you like to look up?",
}


def classify_conversation(prompt: str) -> Optional[str]:
    """Return a conversational kind, or None if the prompt needs the LLM."""
    return _classify_kind(prompt or "")


def pre_route(prompt: str) -> Optional[ParsedIntentResponse]:
    """Return a deterministic conversational intent, or None to fall through to the LLM."""
    kind = _classify_kind(prompt or "")
    if kind is None:
        return None

    if kind == "affirm":
        return ParsedIntentResponse(
            intent_type=IntentType.AFFIRM,
            confidence=1.0,
            action_type=ActionType.CONVERSATION,
            summary=_CANNED["affirm"],
            parameters=IntentParameters(dry_run=False),
            validation=ValidationRequirement(requires_user_approval=False, risk_level="LOW"),
        )
    if kind == "reject":
        return ParsedIntentResponse(
            intent_type=IntentType.REJECT,
            confidence=1.0,
            action_type=ActionType.CONVERSATION,
            summary=_CANNED["reject"],
            parameters=IntentParameters(dry_run=False),
            validation=ValidationRequirement(requires_user_approval=False, risk_level="LOW"),
        )

    return _conversation(_CANNED[kind])
