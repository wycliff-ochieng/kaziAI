from contextvars import ContextVar
from typing import Any, Dict, Optional

import httpx
from tenacity import retry, stop_after_attempt, wait_exponential, retry_if_exception_type
from src.core.config import settings
from src.core.logger import setup_logger

logger = setup_logger("backend_client")

MAX_ROWS_PER_TOOL_RESULT = 20

# Per-request tool routing: the gateway forwards the tenant and ERP type so a
# MOCK conversation never silently reads the production Odoo connector.
_tool_context: ContextVar[Dict[str, str]] = ContextVar("tool_context", default={})


def set_tool_context(tenant_id: str = "", erp_type: str = ""):
    return _tool_context.set({
        "tenant_id": tenant_id or "",
        "erp_type": (erp_type or "").upper(),
    })


def reset_tool_context(token) -> None:
    _tool_context.reset(token)


def _cap_payload(payload: Any) -> Any:
    """Keep list tool outputs small so they fit the 7k-token Groq budget."""
    if isinstance(payload, dict):
        # Common shapes: {"data": [...]} / {"vendors": [...]} / {"invoices": [...]}
        for key in ("data", "vendors", "invoices", "customers", "products", "orders", "items", "results"):
            val = payload.get(key)
            if isinstance(val, list) and len(val) > MAX_ROWS_PER_TOOL_RESULT:
                payload[key] = val[:MAX_ROWS_PER_TOOL_RESULT]
                payload[f"{key}_truncated"] = f"showing {MAX_ROWS_PER_TOOL_RESULT} of {len(val)}"
        return payload
    if isinstance(payload, list) and len(payload) > MAX_ROWS_PER_TOOL_RESULT:
        return payload[:MAX_ROWS_PER_TOOL_RESULT]
    return payload


class BackendClient:
    """Client to interact with the Go backend with resilience built-in."""

    def __init__(self):
        self.base_url = settings.BACKEND_API_URL
        # Configure standard HTTP timeouts (Read, Connect, Pool, Write)
        self.timeout = httpx.Timeout(30.0, connect=10.0)

    def _headers(self) -> Dict[str, str]:
        headers = {"Content-Type": "application/json"}
        # The gateway trusts this shared secret for service-to-service calls and
        # still scopes every request to the tenant forwarded below.
        if settings.INTERNAL_API_TOKEN:
            headers["X-Internal-Token"] = settings.INTERNAL_API_TOKEN
        tenant_id = _tool_context.get().get("tenant_id", "")
        if tenant_id:
            headers["X-Tenant-ID"] = tenant_id
        return headers

    def _params(self, params: Optional[Dict[str, Any]]) -> Dict[str, Any]:
        merged = dict(params or {})
        erp_type = _tool_context.get().get("erp_type", "")
        if erp_type:
            merged["erp_type"] = erp_type
        return merged

    @retry(
        stop=stop_after_attempt(3),
        wait=wait_exponential(multiplier=1, min=2, max=10),
        retry=retry_if_exception_type((httpx.RequestError, httpx.TimeoutException)),
        before_sleep=lambda retry_state: logger.warning(f"Retrying request after error: {retry_state.outcome.exception() if retry_state.outcome else 'Unknown'}")
    )
    async def get(self, endpoint: str, params: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
        async with httpx.AsyncClient(base_url=self.base_url, headers=self._headers(), timeout=self.timeout) as client:
            response = await client.get(endpoint, params=self._params(params))
            if response.status_code >= 400:
                try:
                    payload = response.json()
                    if "error" in payload or "success" in payload:
                        return payload
                except:
                    pass
            response.raise_for_status()
            return _cap_payload(response.json())

    @retry(
        stop=stop_after_attempt(3),
        wait=wait_exponential(multiplier=1, min=2, max=10),
        retry=retry_if_exception_type((httpx.RequestError, httpx.TimeoutException)),
        before_sleep=lambda retry_state: logger.warning(f"Retrying post request after error: {retry_state.outcome.exception() if retry_state.outcome else 'Unknown'}")
    )
    async def post(self, endpoint: str, data: Dict[str, Any]) -> Dict[str, Any]:
        async with httpx.AsyncClient(base_url=self.base_url, headers=self._headers(), timeout=self.timeout) as client:
            response = await client.post(endpoint, params=self._params(None), json=data)
            if response.status_code >= 400:
                try:
                    payload = response.json()
                    if "error" in payload or "success" in payload:
                        return payload
                except:
                    pass
            response.raise_for_status()
            return _cap_payload(response.json())

from langchain_core.tools import tool
from pydantic import BaseModel, Field
import re

def _sanitize_search(search_term: str) -> str:
    """Strip conversational preambles and stop-words so Odoo searches match."""
    s = str(search_term).lower()
    # Remove fragments
    s = re.sub(r"\b(called|named|with the name|with|have|products?|as)\b", "", s)
    # Collapse spaces
    s = " ".join(s.split())
    # Keep up to 4 words max as a heuristic
    return " ".join(s.split()[:4])

class GetCustomersArgs(BaseModel):
    search: str = Field(default="", description="Clean person or company name. Strip conversational preambles like 'called', 'with the name', or 'named'.")
    limit: int = Field(default=20, description="Maximum records to return. Respect user directives like 'first 10', 'top 5'.")

class GetProductsArgs(BaseModel):
    search: str = Field(default="", description="Clean product name or SKU. Strip conversational preambles like 'with the name' or verbs.")
    limit: int = Field(default=20, description="Maximum records to return. Respect user directives like 'first 10', 'top 5'.")

class GetSaleOrdersArgs(BaseModel):
    customer_id: str = Field(default="", description="Odoo res.partner ID. Prefer customer_name when the user gave a name.")
    customer_name: str = Field(default="", description="Customer name exactly as the user said it. The backend resolves it.")
    relationship: str = Field(default="", description="Use 'orders_for_customer' when the user asks for a named customer's orders.")
    state: str = Field(default="", description="Status filter (e.g., 'draft', 'sale', 'done', 'cancel').")
    limit: int = Field(default=20, description="Maximum records to return. Respect 'first 10', 'top 5'.")

class GetInvoicesArgs(BaseModel):
    customer_id: str = Field(default="", description="Odoo res.partner ID.")
    customer_name: str = Field(default="", description="Customer name exactly as the user said it.")
    relationship: str = Field(default="", description="Use 'invoices_for_order' with order_id when the user asks for a document's invoices.")
    order_id: str = Field(default="", description="Sale order ID/number for invoices_for_order.")
    state: str = Field(default="", description="Invoice state: draft, posted, cancel.")
    payment_state: str = Field(default="", description="Payment state: not_paid, partial, paid, in_payment.")
    overdue: bool = Field(default=False, description="True only when the user says overdue/past due. For simply unpaid, use payment_state='not_paid'.")
    limit: int = Field(default=20, description="Maximum records to return.")

class CountRecordsArgs(BaseModel):
    model_name: str = Field(..., description="The model resource to count (e.g., 'customers', 'products', 'orders', 'invoices', 'vendors').")
    search: str = Field(default="", description="Search term or filter to apply.")
    state: str = Field(default="", description="State filter if applicable (e.g., 'posted', 'sale', 'draft').")
    customer_id: str = Field(default="", description="Odoo res.partner ID.")
    customer_name: str = Field(default="", description="Customer name exactly as the user said it. The backend resolves it.")
    payment_state: str = Field(default="", description="Invoice payment state: not_paid, partial, paid, in_payment.")
    overdue: bool = Field(default=False, description="True only for overdue/past-due invoices. For unpaid, use payment_state='not_paid'.")

class GetVendorsArgs(BaseModel):
    search: str = Field(default="", description="Clean vendor or company name (1-4 words). NEVER pass verbs, products, or sentence fragments.")
    limit: int = Field(default=20, description="Maximum records to return.")

class CreateSaleOrderArgs(BaseModel):
    customer_id: str = Field(default="", description="Odoo res.partner ID. If unknown, leave empty and pass customer_name instead.")
    customer_name: str = Field(default="", description="Customer name fallback. Tool will auto-resolve this to an ID.")
    product_id: str = Field(default="", description="Odoo product.product ID. If unknown, leave empty and pass product_name.")
    product_name: str = Field(default="", description="Product name fallback. Tool will auto-resolve this to an ID.")
    quantity: float = Field(default=1.0, description="Quantity to purchase.")
    unit_price: float = Field(default=0.0, description="Optional unit price override.")
    deliver_immediately: bool = Field(default=False, description="Set to true if user instructs to deliver/ship the products right away.")

class CreateRFQArgs(BaseModel):
    vendor_id: str = Field(default="", description="Odoo res.partner Vendor ID. If unknown, leave empty and pass vendor_name instead.")
    vendor_name: str = Field(default="", description="Vendor name fallback. Tool will auto-resolve this to an ID.")
    product_id: str = Field(default="", description="Odoo product.product ID. If unknown, leave empty and pass product_name.")
    product_name: str = Field(default="", description="Product name fallback. Tool will auto-resolve this to an ID.")
    quantity: float = Field(default=1.0, description="Quantity to purchase.")
    unit_price: float = Field(default=0.0, description="Optional unit price override.")


backend_client = BackendClient()

@tool(args_schema=GetCustomersArgs)
async def get_customers_tool(search: str = "", limit: int = 20) -> dict:
    """Fetch customers. `search` MUST be ONLY the person/company name. NEVER pass verbs, products, or sentence fragments."""
    try:
        params = {}
        if search: params["search"] = search
        if limit: params["limit"] = limit
        return await backend_client.get("/api/v1/sales/customers", params=params if params else None)
    except Exception as e:
        return {"error": str(e)}

@tool(args_schema=GetProductsArgs)
async def get_products_tool(search: str = "", limit: int = 20) -> dict:
    """Fetch products. `search` MUST be ONLY the product name/SKU."""
    try:
        params = {}
        if search: params["search"] = search
        if limit: params["limit"] = limit
        return await backend_client.get("/api/v1/sales/products", params=params if params else None)
    except Exception as e:
        return {"error": str(e)}

@tool(args_schema=GetSaleOrdersArgs)
async def get_sale_orders_tool(customer_id: str = "", customer_name: str = "", relationship: str = "", state: str = "", limit: int = 20) -> dict:
    """Fetch sale orders. Pass customer_name for a named customer; never invent an ID."""
    try:
        params = {}
        if customer_id: params["customer_id"] = customer_id
        if customer_name: params["customer_name"] = customer_name
        if relationship: params["relationship"] = relationship
        if state: params["state"] = state
        if limit: params["limit"] = limit
        return await backend_client.get("/api/v1/sales/orders", params=params if params else None)
    except Exception as e:
        return {"error": str(e)}

@tool(args_schema=GetInvoicesArgs)
async def get_invoices_tool(customer_id: str = "", customer_name: str = "", relationship: str = "", order_id: str = "", state: str = "", payment_state: str = "", overdue: bool = False, limit: int = 20) -> dict:
    """Fetch invoices. Unpaid = payment_state 'not_paid'; overdue = overdue true (past due). Pass customer_name for a named customer."""
    try:
        params = {}
        if customer_id: params["customer_id"] = customer_id
        if customer_name: params["customer_name"] = customer_name
        if relationship: params["relationship"] = relationship
        if order_id: params["order_id"] = order_id
        if state: params["state"] = state
        if payment_state: params["payment_state"] = payment_state
        if overdue: params["overdue"] = "true"
        if limit: params["limit"] = limit
        return await backend_client.get("/api/v1/sales/invoices", params=params if params else None)
    except Exception as e:
        return {"error": str(e)}

@tool(args_schema=CountRecordsArgs)
async def count_records_tool(model_name: str, search: str = "", state: str = "", customer_id: str = "", customer_name: str = "", payment_state: str = "", overdue: bool = False) -> dict:
    """Use this tool when the user asks 'how many' records exist instead of listing them. Pass every filter the user stated."""
    try:
        params = {}
        if search: params["search"] = search
        if state: params["state"] = state
        if customer_id: params["customer_id"] = customer_id
        if customer_name: params["customer_name"] = customer_name
        if payment_state: params["payment_state"] = payment_state
        if overdue: params["overdue"] = "true"

        # Maps logical model name to the correct API endpoint
        endpoints = {
            "customers": "/api/v1/sales/customers/count",
            "products": "/api/v1/sales/products/count",
            "orders": "/api/v1/sales/orders/count",
            "invoices": "/api/v1/sales/invoices/count",
            "vendors": "/api/v1/purchase/vendors/count"
        }

        endpoint = endpoints.get(model_name.lower())
        if not endpoint:
            # Fallback to fetching minimal dataset and counting length
            return {"error": f"Count operation not mapped for: {model_name}. Try fetching a list tool directly if supported."}

        return await backend_client.get(endpoint, params=params if params else None)
    except Exception as e:
        return {"error": str(e)}


@tool(args_schema=CreateSaleOrderArgs)
async def create_sale_order_tool(customer_id: str = "", customer_name: str = "", product_id: str = "", product_name: str = "", quantity: float = 1.0, unit_price: float = 0.0, deliver_immediately: bool = False) -> dict:
    """Creates a new sales order or quotation for a customer with a specified product.
       Automatically performs atomic entity resolution if names are passed instead of IDs."""
    try:
        from src.tools.backend_client import get_customers_tool, get_products_tool

        # Disambiguate Customer
        if not customer_id and customer_name:
            res = await get_customers_tool.ainvoke({"search": customer_name, "limit": 2})
            data = res.get("data", [])
            if len(data) == 1:
                customer_id = str(data[0].get("id", ""))
            elif len(data) > 1:
                return {"error": {"code": "AMBIGUOUS_ENTITY", "message": f"Found multiple customers matching '{customer_name}'", "data": data}}
            else:
                return {"error": {"code": "ENTITY_NOT_FOUND", "message": f"No customer found matching '{customer_name}'"}}

        # Disambiguate Product
        if not product_id and product_name:
            res = await get_products_tool.ainvoke({"search": product_name, "limit": 2})
            data = res.get("data", [])
            if len(data) == 1:
                product_id = str(data[0].get("id", ""))
            elif len(data) > 1:
                return {"error": {"code": "AMBIGUOUS_ENTITY", "message": f"Found multiple products matching '{product_name}'", "data": data}}
            else:
                return {"error": {"code": "ENTITY_NOT_FOUND", "message": f"No product found matching '{product_name}'"}}

        if not customer_id or not product_id:
            return {"error": {"message": "Both customer_id and product_id are required after resolution."}}

        payload = {
            "customer_id": customer_id,
            "lines": [
                {"product_id": product_id, "quantity": quantity, "unit_price": unit_price}
            ],
            # Always preview. Execution only happens after server-side approval.
            "dry_run": True,
        }
        res = await backend_client.post("/api/v1/sales/orders", data=payload)
        if deliver_immediately:
            res["requested_delivery"] = True
        return res
    except Exception as e:
        return {"error": str(e)}

@tool
async def confirm_sale_order_tool(order_id: str) -> dict:
    """Preview confirming a sale order (draft/sent -> sale). Never executes."""
    try:
        return await backend_client.post(f"/api/v1/sales/orders/{order_id}/confirm?dry_run=true", data={})
    except Exception as e:
        return {"error": str(e)}

@tool
async def deliver_sale_order_tool(order_id: str) -> dict:
    """Preview delivering a sale order (validate its picking). Never executes."""
    try:
        return await backend_client.post(f"/api/v1/sales/orders/{order_id}/deliver?dry_run=true", data={})
    except Exception as e:
        return {"error": str(e)}

@tool
async def create_invoice_tool(order_id: str) -> dict:
    """Preview creating a draft invoice for a confirmed sales order. Never executes."""
    try:
        return await backend_client.post(f"/api/v1/sales/orders/{order_id}/invoice?dry_run=true", data={})
    except Exception as e:
        return {"error": str(e)}

@tool
async def post_invoice_tool(invoice_id: str) -> dict:
    """Preview posting an invoice. Never executes."""
    try:
        return await backend_client.post(f"/api/v1/sales/invoices/{invoice_id}/post?dry_run=true", data={})
    except Exception as e:
        return {"error": str(e)}

# ---- Purchase Operations (canonical; ERP mapping lives in Go adapters) ----

@tool(args_schema=GetVendorsArgs)
async def get_vendors_tool(search: str = "", limit: int = 20) -> dict:
    """Fetch vendors. `search` MUST be ONLY the vendor/company name."""
    try:
        params = {}
        if search: params["search"] = search
        if limit: params["limit"] = limit
        return await backend_client.get("/api/v1/purchase/vendors", params=params if params else None)
    except Exception as e:
        return {"error": str(e)}

@tool
async def get_purchase_orders_tool(vendor_id: str = "", state: str = "") -> dict:
    """Fetch RFQs/purchase orders. Filter by vendor_id and/or state (draft/purchase/done/cancel). Use to verify current state before confirm/receive/bill."""
    try:
        params = {}
        if vendor_id:
            params["vendor_id"] = vendor_id
        if state:
            params["state"] = state
        return await backend_client.get("/api/v1/purchase/orders", params=params or None)
    except Exception as e:
        return {"error": str(e)}

@tool
async def get_purchase_order_by_id_tool(order_id: str) -> dict:
    """Fetch a single RFQ/purchase order by ID or order number. Always call before confirm/receive/bill/cancel to verify the transition is legal."""
    try:
        return await backend_client.get(f"/api/v1/purchase/orders/{order_id}")
    except Exception as e:
        return {"error": str(e)}

@tool(args_schema=CreateRFQArgs)
async def create_rfq_tool(vendor_id: str = "", vendor_name: str = "", product_id: str = "", product_name: str = "", quantity: float = 1.0, unit_price: float = 0.0) -> dict:
    """Draft a new RFQ for a vendor with a single line item.
       Automatically performs atomic entity resolution if names are passed instead of IDs."""
    try:
        from src.tools.backend_client import get_vendors_tool, get_products_tool

        # Disambiguate Vendor
        if not vendor_id and vendor_name:
            res = await get_vendors_tool.ainvoke({"search": vendor_name, "limit": 2})
            data = res.get("data", [])
            if len(data) == 1:
                vendor_id = str(data[0].get("id", ""))
            elif len(data) > 1:
                return {"error": {"code": "AMBIGUOUS_ENTITY", "message": f"Found multiple vendors matching '{vendor_name}'", "data": data}}
            else:
                return {"error": {"code": "ENTITY_NOT_FOUND", "message": f"No vendor found matching '{vendor_name}'"}}

        # Disambiguate Product
        if not product_id and product_name:
            res = await get_products_tool.ainvoke({"search": product_name, "limit": 2})
            data = res.get("data", [])
            if len(data) == 1:
                product_id = str(data[0].get("id", ""))
            elif len(data) > 1:
                return {"error": {"code": "AMBIGUOUS_ENTITY", "message": f"Found multiple products matching '{product_name}'", "data": data}}
            else:
                return {"error": {"code": "ENTITY_NOT_FOUND", "message": f"No product found matching '{product_name}'"}}

        if not vendor_id or not product_id:
            return {"error": {"message": "Both vendor_id and product_id are required after resolution."}}

        payload = {
            "vendor_id": vendor_id,
            "lines": [
                {"product_id": product_id, "quantity": quantity, "unit_price": unit_price}
            ],
            # Always preview. Execution only happens after server-side approval.
            "dry_run": True,
        }
        return await backend_client.post("/api/v1/purchase/orders", data=payload)
    except Exception as e:
        return {"error": str(e)}

@tool
async def confirm_purchase_order_tool(order_id: str) -> dict:
    """Preview confirming an RFQ (draft -> purchase). Never executes."""
    try:
        return await backend_client.post(f"/api/v1/purchase/orders/{order_id}/confirm?dry_run=true", data={})
    except Exception as e:
        return {"error": str(e)}

@tool
async def receive_purchase_order_tool(order_id: str) -> dict:
    """Preview receiving a confirmed PO (full receipt only in v1). Never executes."""
    try:
        return await backend_client.post(f"/api/v1/purchase/orders/{order_id}/receive?dry_run=true", data={})
    except Exception as e:
        return {"error": str(e)}

@tool
async def create_vendor_bill_tool(order_id: str) -> dict:
    """Preview creating a vendor bill from a confirmed/received PO. Never executes."""
    try:
        return await backend_client.post(f"/api/v1/purchase/orders/{order_id}/bill?dry_run=true", data={})
    except Exception as e:
        return {"error": str(e)}

@tool
async def post_vendor_bill_tool(bill_id: str) -> dict:
    """Preview posting a vendor bill. Never executes."""
    try:
        return await backend_client.post(f"/api/v1/purchase/bills/{bill_id}/post?dry_run=true", data={})
    except Exception as e:
        return {"error": str(e)}

@tool
async def cancel_purchase_order_tool(order_id: str) -> dict:
    """Preview cancelling a draft RFQ. Never executes."""
    try:
        return await backend_client.post(f"/api/v1/purchase/orders/{order_id}/cancel?dry_run=true", data={})
    except Exception as e:
        return {"error": str(e)}
