"""Tests for the controlled natural-language -> Odoo domain compiler.

These cover the exact log failures: state filters being ignored
("invoices in paid state") and relationship filters being dropped
("customers with more than one sale order").
"""
from src.schemas.intents import QueryFilter
from src.services.filter_compiler import (
    allowed_fields,
    compile_filters,
    compile_odoo_domain,
)


def test_numeric_relationship_filter_compiles():
    res = compile_filters("customers", [QueryFilter(field="sale_order_count", operator=">", value="1")])
    assert res.errors == []
    assert res.domain == [("sale_order_count", ">", 1)]
    assert isinstance(res.domain[0][2], int)


def test_invoice_state_filter_compiles():
    res = compile_filters("invoices", [QueryFilter(field="payment_state", operator="=", value="paid")])
    assert res.domain == [("payment_state", "=", "paid")]


def test_multiple_filters_and_conjunction_defaults_to_and():
    res = compile_filters(
        "invoices",
        [QueryFilter(field="state", operator="=", value="posted"), QueryFilter(field="amount_total", operator=">", value="500")],
    )
    assert res.domain == [("state", "=", "posted"), ("amount_total", ">", 500.0)]
    assert not any(item == "|" for item in res.domain)


def test_or_conjunction_prefixes_polish_operators():
    res = compile_filters(
        "orders",
        [QueryFilter(field="state", operator="=", value="draft"), QueryFilter(field="state", operator="=", value="sale")],
        conjunction="OR",
    )
    assert res.domain[0] == "|"
    assert res.domain[1] == ("state", "=", "draft")
    assert res.domain[2] == ("state", "=", "sale")


def test_in_operator_accepts_comma_list():
    res = compile_filters("orders", [QueryFilter(field="state", operator="in", value="draft,sale")])
    assert res.domain == [("state", "in", ["draft", "sale"])]


def test_unknown_field_is_rejected_not_passed_through():
    res = compile_filters("customers", [QueryFilter(field="password", operator="=", value="x")])
    assert res.domain == []
    assert any("not filterable" in e for e in res.errors)


def test_disallowed_operator_is_rejected():
    res = compile_filters("customers", [QueryFilter(field="name", operator=">", value="bob")])
    assert res.domain == []
    assert any("not allowed" in e for e in res.errors)


def test_bad_value_type_is_reported():
    res = compile_filters("customers", [QueryFilter(field="balance", operator=">", value="lots")])
    assert res.domain == []
    assert any("not a valid" in e for e in res.errors)


def test_unknown_resource_is_reported():
    res = compile_filters("spaceships", [QueryFilter(field="id", operator="=", value="1")])
    assert any("unknown resource" in e for e in res.errors)


def test_operator_aliases_normalise():
    res = compile_filters("customers", [QueryFilter(field="name", operator="contains", value="acme")])
    assert res.domain == [("name", "ilike", "acme")]


def test_plain_dict_filters_are_supported():
    res = compile_filters("products", [{"field": "qty_available", "operator": "<=", "value": 0}])
    assert res.domain == [("qty_available", "<=", 0.0)]


def test_allowlist_exposes_relationship_counts():
    fields = allowed_fields("customers")
    assert "sale_order_count" in fields
    assert "invoice_count" in fields


def test_legacy_compile_odoo_domain():
    domain = compile_odoo_domain([{"field": "state", "operator": "=", "value": "paid"}])
    assert domain == [("state", "=", "paid")]
    assert compile_odoo_domain([]) == []
