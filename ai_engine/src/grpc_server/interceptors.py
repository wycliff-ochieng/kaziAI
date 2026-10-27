import grpc
from typing import Callable, Any

class TenantContextInterceptor(grpc.aio.ServerInterceptor):
    async def intercept_service(
        self,
        continuation: Callable,
        handler_call_details: grpc.HandlerCallDetails,
    ) -> grpc.RpcMethodHandler:
        """
        Intercepts incoming gRPC calls to extract the X-Tenant-ID from headers
        and inject it into the application context safely.
        """
        metadata = dict(handler_call_details.invocation_metadata)
        tenant_id = metadata.get("x-tenant-id", "default")
        
        # In a real app we set contextvars here to isolate the tenant
        # contextvars_tenant.set(tenant_id)
        
        return await continuation(handler_call_details)
