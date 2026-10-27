from fastapi import APIRouter, HTTPException, Depends
from langchain_core.messages import AIMessage, HumanMessage, ToolMessage

from src.schemas.intents import ParseIntentRequest, ParsedIntentResponse
from src.services.intent_service import intent_service
from src.services.session_memory import session_memory
from src.tools.backend_client import reset_tool_context, set_tool_context
from src.services.turn_plan import build_turn_plan
from src.api.dependencies import get_agent

router = APIRouter(prefix="/api/v1", tags=["AI Engine"])


def _text(content) -> str:
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        parts = []
        for block in content:
            if isinstance(block, str):
                parts.append(block)
            elif isinstance(block, dict) and block.get("text"):
                parts.append(str(block["text"]))
        return "".join(parts)
    return str(content or "")


def _turn_messages(messages):
    """Only inspect the current turn; checkpointer history must not leak plans."""
    turn = []
    for message in reversed(messages or []):
        turn.append(message)
        if isinstance(message, HumanMessage):
            break
    turn.reverse()
    return turn


def _thread_id(request: ParseIntentRequest) -> str:
    if request.thread_id:
        return request.thread_id
    tenant = request.tenant_id or "default_tenant"
    conversation = request.conversation_id or "default"
    return f"{tenant}:{conversation}"


@router.post("/parse-intent", response_model=ParsedIntentResponse)
async def parse_intent(request: ParseIntentRequest):
    try:
        return intent_service.parse_intent(request)
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Failed to parse intent: {str(e)}")


@router.post("/chat")
async def chat_intent(request: ParseIntentRequest):
    try:
        parsed = intent_service.parse_intent(request)
        return {
            "status": "success",
            "prompt": request.prompt,
            "parsed_intent": parsed,
        }
    except Exception as e:
        raise HTTPException(status_code=500, detail=f"Chat execution failed: {str(e)}")


@router.post("/process")
async def process_natural_language(request: ParseIntentRequest, agent=Depends(get_agent)):
    try:
        prompt = (request.prompt or "").strip()
        # Guard: never send huge prompts to a small-context model tier.
        if len(prompt) > 2000:
            prompt = prompt[:2000]

        thread_id = _thread_id(request)
        tenant_id = request.tenant_id or "default_tenant"
        config = {"configurable": {"thread_id": thread_id}}
        ctx_token = set_tool_context(tenant_id, request.erp_type or "")
        try:
            result = await agent.ainvoke(
                {
                    "messages": [HumanMessage(content=prompt)],
                    "tenant_id": tenant_id,
                    "user_id": "ws",
                    "channel": "WEB",
                },
                config=config,
            )
        finally:
            reset_tool_context(ctx_token)

        messages = _turn_messages(result.get("messages", []) or [])
        tool_calls = []
        tool_results = []
        last_ai_text = ""
        for message in messages:
            if isinstance(message, AIMessage):
                for call in getattr(message, "tool_calls", None) or []:
                    if isinstance(call, dict):
                        tool_calls.append({"name": call.get("name"), "args": call.get("args") or {}})
                text = _text(message.content)
                if text.strip() and not getattr(message, "tool_calls", None):
                    last_ai_text = text
            elif isinstance(message, ToolMessage):
                raw = (getattr(message, "additional_kwargs", None) or {}).get("raw_content")
                tool_results.append({
                    "name": getattr(message, "name", "") or "",
                    "content": raw if raw is not None else message.content,
                })

        plan = build_turn_plan(tool_calls, tool_results)

        # Record the *real* result frame only after execution.
        if plan["action_type"] == "READ" and plan["data"] is not None:
            session_memory.record_query(
                thread_id=thread_id,
                tenant_id=tenant_id,
                intent_type=plan["intent_type"],
                resource=plan["resource"],
                record_ids=plan["record_ids"],
                result_count=plan["count"],
                summary=last_ai_text,
            )

        response_text = last_ai_text.strip()
        if not response_text:
            response_text = result.get("final_response_text") or _text(messages[-1].content if messages else "")
        if not response_text.strip():
            response_text = "I couldn't find anything for that request."

        return {
            "status": "success",
            "prompt": request.prompt,
            "response": response_text,
            "thread_id": thread_id,
            **plan,
        }
    except Exception as e:
        msg = str(e)
        if "413" in msg or "rate_limit_exceeded" in msg or "Request too large" in msg:
            raise HTTPException(
                status_code=429,
                detail="Conversation grew too large for the model's token budget. Start a new chat (fresh conversation_id) and retry with a shorter request.",
            )
        raise HTTPException(status_code=500, detail=f"Agent execution failed: {msg}")


@router.post("/reset")
async def reset_conversation(request: ParseIntentRequest, agent=Depends(get_agent)):
    """Best-effort reset hint: clients should switch to a fresh thread_id."""
    return {
        "status": "success",
        "thread_id": _thread_id(request),
        "hint": "Start a new chat with a fresh conversation_id/thread_id to clear token buildup.",
    }
