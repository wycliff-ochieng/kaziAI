# Placeholder for compiled gRPC implementations
# Commonly you would import pb2 and pb2_grpc files here.

import grpc
from src.core.logger import setup_logger
from src.agents.erp_agent import get_erp_agent
from langchain_core.messages import HumanMessage

logger = setup_logger("grpc_services")

class AIEngineServiceContext:
    """Mock implementation wrapper of the compiled gRPC servicer."""
    async def ProcessIntent(self, request, context: grpc.aio.ServicerContext):
        # Extract tenant from context metadata map (injected via interceptor)
        tenant_id = dict(context.invocation_metadata()).get("x-tenant-id", "default")
        
        logger.info(f"Processing gRPC intent for tenant {tenant_id}")
        agent = get_erp_agent()
        
        result = await agent.ainvoke({"messages": [HumanMessage(content=request.prompt)], "tenant_id": tenant_id})
        
        # Map back to protobuf format
        # return ai_engine_pb2.ProcessIntentResponse(
        #     response_text=result["messages"][-1].content
        # )
        return {"response_text": result["messages"][-1].content}
