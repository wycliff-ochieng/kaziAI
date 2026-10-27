"""Session state-frame memory for multi-turn ERP chaining.

Blindly re-sending every prior tool result to the model blows the token budget
and produces hallucinated arithmetic. Instead we keep a *compact* frame of what
the previous turn actually established — target resource, the record IDs the
user saw, the domain that produced them, and the count — and feed only that
back to the model so anaphora ("from those", "the first 10") is grounded in
real data.

Frames are process-local and expire so stale database IDs can't poison later
workflows. The store is deliberately dependency-free; if multi-process
deployment needs sharing this moves behind an interface (e.g. Redis).
"""
from __future__ import annotations

import threading
import time
from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional

from src.core.config import settings


@dataclass
class SessionState:
    thread_id: str
    tenant_id: str = "default_tenant"

    # Last executed query frame
    last_intent_type: str = ""
    last_resource: str = ""
    last_record_ids: List[str] = field(default_factory=list)
    last_result_count: int = 0
    last_filters: List[Dict[str, Any]] = field(default_factory=list)
    last_domain: List[Any] = field(default_factory=list)
    last_summary: str = ""

    # A held write awaiting confirmation ("okay" / "cancel that")
    pending_intent_type: str = ""
    pending_prompt: str = ""

    updated_at: float = field(default_factory=time.time)


class SessionMemory:
    def __init__(self, ttl_seconds: Optional[int] = None) -> None:
        self._ttl = ttl_seconds if ttl_seconds is not None else settings.SESSION_TTL_SECONDS
        self._lock = threading.Lock()
        self._store: Dict[str, SessionState] = {}

    @staticmethod
    def _key(tenant_id: str, thread_id: str) -> str:
        return f"{tenant_id or 'default_tenant'}:{thread_id or 'default_thread'}"

    def _evict_locked(self) -> None:
        now = time.time()
        expired = [k for k, v in self._store.items() if now - v.updated_at > self._ttl]
        for k in expired:
            self._store.pop(k, None)

    def get(self, thread_id: str, tenant_id: str = "default_tenant") -> SessionState:
        key = self._key(tenant_id, thread_id)
        with self._lock:
            self._evict_locked()
            state = self._store.get(key)
            if state is None:
                state = SessionState(thread_id=thread_id or "default_thread", tenant_id=tenant_id or "default_tenant")
                self._store[key] = state
            return state

    def record_query(
        self,
        thread_id: str,
        tenant_id: str,
        intent_type: str,
        resource: str = "",
        record_ids: Optional[List[str]] = None,
        result_count: int = 0,
        filters: Optional[List[Dict[str, Any]]] = None,
        domain: Optional[List[Any]] = None,
        summary: str = "",
        limit: int = 50,
    ) -> SessionState:
        state = self.get(thread_id, tenant_id)
        with self._lock:
            state.last_intent_type = intent_type
            if resource:
                state.last_resource = resource
            ids = [str(r) for r in (record_ids or [])]
            state.last_record_ids = ids[:limit]
            state.last_result_count = result_count or len(ids)
            state.last_filters = filters or []
            state.last_domain = domain or []
            if summary:
                state.last_summary = summary
            state.updated_at = time.time()
        return state

    def set_pending(self, thread_id: str, tenant_id: str, intent_type: str, prompt: str) -> None:
        state = self.get(thread_id, tenant_id)
        with self._lock:
            state.pending_intent_type = intent_type
            state.pending_prompt = prompt
            state.updated_at = time.time()

    def clear_pending(self, thread_id: str, tenant_id: str) -> None:
        state = self.get(thread_id, tenant_id)
        with self._lock:
            state.pending_intent_type = ""
            state.pending_prompt = ""
            state.updated_at = time.time()

    def clear(self, thread_id: str, tenant_id: str = "default_tenant") -> None:
        key = self._key(tenant_id, thread_id)
        with self._lock:
            self._store.pop(key, None)

    def describe(self, thread_id: str, tenant_id: str = "default_tenant") -> str:
        """Compact, token-cheap frame to inject into the model's context."""
        state = self.get(thread_id, tenant_id)
        if not state.last_intent_type and not state.pending_intent_type:
            return "No prior turns in this conversation."

        parts: List[str] = []
        if state.last_intent_type:
            bits = [f"last_operation={state.last_intent_type}"]
            if state.last_resource:
                bits.append(f"resource={state.last_resource}")
            bits.append(f"matched={state.last_result_count}")
            if state.last_record_ids:
                sample = ", ".join(state.last_record_ids[:10])
                bits.append(f"visible_record_ids=[{sample}]")
            if state.last_filters:
                rendered = "; ".join(
                    f"{f.get('field')} {f.get('operator')} {f.get('value')}" for f in state.last_filters
                )
                bits.append(f"active_filters=({rendered})")
            if state.last_domain:
                bits.append(f"active_domain={state.last_domain}")
            parts.append("Previous result frame: " + " ".join(bits) + ".")
        if state.pending_intent_type:
            parts.append(
                f"A {state.pending_intent_type} action is awaiting the user's confirmation "
                f"(original request: {state.pending_prompt!r})."
            )
        return " ".join(parts)


session_memory = SessionMemory()
