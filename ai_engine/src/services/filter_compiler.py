"""Controlled natural-language -> Odoo domain compiler.

The LLM must never invent arbitrary Odoo queries. It emits flat, typed
constraints (``field``, ``operator``, ``value``); this module validates them
against a per-resource field allowlist and compiles a valid Odoo *Polish
notation* domain. Unknown fields/operators are rejected rather than passed to
the database.
"""
from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional

#: Logical resource -> Odoo model.
RESOURCE_MODEL: Dict[str, str] = {
    "customers": "res.partner",
    "vendors": "res.partner",
    "products": "product.product",
    "orders": "sale.order",
    "purchase_orders": "purchase.order",
    "invoices": "account.move",
    "deliveries": "stock.picking",
}

#: value type -> python caster
_TYPES = {
    "int": lambda v: int(float(v)),
    "float": lambda v: float(v),
    "bool": lambda v: str(v).strip().lower() in ("1", "true", "yes", "y"),
    "str": lambda v: str(v),
    "date": lambda v: str(v),
}

_COMPARISON_OPS = {"=", "!=", ">", ">=", "<", "<=", "like", "ilike", "not like", "not ilike", "in", "not in"}
_LIST_OPS = {"in", "not in"}


def _f(value_type: str, *operators: str) -> Dict[str, Any]:
    return {"type": value_type, "ops": set(operators)}


#: Allowlisted fields per resource. Keep this tight: it is the security boundary
#: between an LLM utterance and the ERP database.
RESOURCE_FIELDS: Dict[str, Dict[str, Dict[str, Any]]] = {
    "customers": {
        "id": _f("int", "=", "!=", "in", "not in"),
        "name": _f("str", "=", "!=", "ilike", "like", "not ilike"),
        "email": _f("str", "=", "!=", "ilike"),
        "phone": _f("str", "=", "!=", "ilike"),
        "balance": _f("float", "=", "!=", ">", ">=", "<", "<="),
        "credit_limit": _f("float", "=", "!=", ">", ">=", "<", "<="),
        "sale_order_count": _f("int", "=", "!=", ">", ">=", "<", "<="),
        "invoice_count": _f("int", "=", "!=", ">", ">=", "<", "<="),
        "city": _f("str", "=", "ilike"),
        "create_date": _f("date", ">=", "<=", ">", "<"),
    },
    "vendors": {
        "id": _f("int", "=", "!=", "in", "not in"),
        "name": _f("str", "=", "!=", "ilike", "like", "not ilike"),
        "email": _f("str", "=", "!=", "ilike"),
        "phone": _f("str", "=", "!=", "ilike"),
        "balance": _f("float", "=", "!=", ">", ">=", "<", "<="),
    },
    "products": {
        "id": _f("int", "=", "!=", "in", "not in"),
        "name": _f("str", "=", "!=", "ilike", "like", "not ilike"),
        "default_code": _f("str", "=", "!=", "ilike", "like"),
        "list_price": _f("float", "=", "!=", ">", ">=", "<", "<="),
        "qty_available": _f("float", "=", "!=", ">", ">=", "<", "<="),
    },
    "orders": {
        "id": _f("int", "=", "!=", "in", "not in"),
        "name": _f("str", "=", "!=", "ilike", "like"),
        "partner_id": _f("int", "=", "!=", "in", "not in"),
        "state": _f("str", "=", "!=", "in", "not in"),
        "amount_total": _f("float", "=", "!=", ">", ">=", "<", "<="),
        "date_order": _f("date", ">=", "<=", ">", "<"),
        "invoice_status": _f("str", "=", "!=", "in", "not in"),
    },
    "purchase_orders": {
        "id": _f("int", "=", "!=", "in", "not in"),
        "name": _f("str", "=", "!=", "ilike", "like"),
        "partner_id": _f("int", "=", "!=", "in", "not in"),
        "state": _f("str", "=", "!=", "in", "not in"),
        "amount_total": _f("float", "=", "!=", ">", ">=", "<", "<="),
        "date_order": _f("date", ">=", "<=", ">", "<"),
    },
    "invoices": {
        "id": _f("int", "=", "!=", "in", "not in"),
        "name": _f("str", "=", "!=", "ilike", "like"),
        "partner_id": _f("int", "=", "!=", "in", "not in"),
        "state": _f("str", "=", "!=", "in", "not in"),
        "payment_state": _f("str", "=", "!=", "in", "not in"),
        "move_type": _f("str", "=", "!=", "in", "not in"),
        "amount_total": _f("float", "=", "!=", ">", ">=", "<", "<="),
        "amount_residual": _f("float", "=", "!=", ">", ">=", "<", "<="),
        "invoice_date": _f("date", ">=", "<=", ">", "<"),
        "due_date": _f("date", ">=", "<=", ">", "<"),
    },
    "deliveries": {
        "id": _f("int", "=", "!=", "in", "not in"),
        "name": _f("str", "=", "!=", "ilike", "like"),
        "sale_id": _f("int", "=", "!=", "in", "not in"),
        "partner_id": _f("int", "=", "!=", "in", "not in"),
        "state": _f("str", "=", "!=", "in", "not in"),
        "picking_type_code": _f("str", "=", "!=", "in", "not in"),
    },
}

#: Normalise operator spellings the model might emit.
_OP_ALIASES = {
    "==": "=",
    "equals": "=",
    "eq": "=",
    "gte": ">=",
    "lte": "<=",
    "gt": ">",
    "lt": "<",
    "contains": "ilike",
    "not contains": "not ilike",
}


@dataclass
class CompileResult:
    domain: List[Any] = field(default_factory=list)
    errors: List[str] = field(default_factory=list)


def allowed_fields(resource: str) -> List[str]:
    return sorted(RESOURCE_FIELDS.get((resource or "").lower(), {}).keys())


def allowed_fields_prompt(resource: str) -> str:
    """Human/LLM-readable allowlist for the classifier prompt."""
    fields = RESOURCE_FIELDS.get((resource or "").lower())
    if not fields:
        return ""
    parts = [f"{name} ({spec['type']}: {', '.join(sorted(spec['ops']))})" for name, spec in sorted(fields.items())]
    return "; ".join(parts)


def _cast(value_type: str, value: Any, operator: str) -> Any:
    caster = _TYPES[value_type]
    if operator in _LIST_OPS:
        if isinstance(value, (list, tuple)):
            return [caster(v) for v in value]
        # "in" arrives as a comma-separated string from the model.
        return [caster(v.strip()) for v in str(value).split(",") if str(v).strip() != ""]
    return caster(value)


def compile_filters(resource: str, filters: List[Any], conjunction: str = "AND") -> CompileResult:
    """Validate + compile flat filters into an Odoo domain.

    ``filters`` may contain :class:`QueryFilter` models or plain dicts.
    """
    result = CompileResult()
    resource = (resource or "").lower()
    schema = RESOURCE_FIELDS.get(resource)
    if schema is None:
        result.errors.append(f"unknown resource '{resource}'")
        return result

    tuples: List[tuple] = []
    for raw in filters or []:
        if hasattr(raw, "model_dump"):
            raw = raw.model_dump()
        if not isinstance(raw, dict):
            result.errors.append(f"filter is not an object: {raw!r}")
            continue

        fname = str(raw.get("field") or "").strip()
        op = str(raw.get("operator") or "=").strip().lower()
        op = _OP_ALIASES.get(op, op)
        value = raw.get("value")

        spec = schema.get(fname)
        if spec is None:
            result.errors.append(f"field '{fname}' is not filterable on {resource}")
            continue
        if op not in spec["ops"]:
            result.errors.append(f"operator '{op}' not allowed for {resource}.{fname}")
            continue
        if value is None or (isinstance(value, str) and value.strip() == ""):
            result.errors.append(f"missing value for {resource}.{fname}")
            continue

        try:
            tuples.append((fname, op, _cast(spec["type"], value, op)))
        except (TypeError, ValueError):
            result.errors.append(f"value {value!r} is not a valid {spec['type']} for {resource}.{fname}")

    if not tuples:
        return result

    conj = "OR" if (conjunction or "AND").strip().upper() == "OR" else "AND"
    domain: List[Any] = []
    if conj == "OR" and len(tuples) > 1:
        domain.extend(["|"] * (len(tuples) - 1))
    domain.extend(tuples)
    result.domain = domain
    return result


def compile_odoo_domain(filters: List[Any], conjunction: str = "AND") -> List[Any]:
    """Legacy/utility entry point kept for callers that only need tuple building."""
    domain: List[Any] = []
    if not filters:
        return domain
    if conjunction.strip().upper() == "OR" and len(filters) > 1:
        domain.extend(["|"] * (len(filters) - 1))
    for cond in filters:
        if hasattr(cond, "model_dump"):
            cond = cond.model_dump()
        op = str(cond.get("operator", "=")).strip().lower()
        op = _OP_ALIASES.get(op, op)
        domain.append((cond["field"], op, cond.get("value")))
    return domain
