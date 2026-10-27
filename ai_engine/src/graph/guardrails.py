from src.graph.state import AgentState, DryRunPreview

async def guardrail_node(state: AgentState) -> dict:
    """
    Evaluates agent output. If a write tool was selected, encapsulates it into a 
    DryRunPreview rather than executing it directly.
    """
    pending_calls = state.get("pending_tool_calls", [])
    
    WRITE_TOOLS = ["transfer_stock", "create_purchase_order", "reconcile_invoice", "update_customer", "create_sale_order"]
    
    for call in pending_calls:
        if call.get("tool_name") in WRITE_TOOLS:
            # Convert raw tool request into a safe Dry-Run Impact Payload
            dry_run = DryRunPreview(
                action_type=call["tool_name"],
                target_module=state.get("intent_category", "UNKNOWN"),
                impact_summary=f"Requesting execution of {call['tool_name']} with args {call.get('args', {})}",
                affected_records_count=1,
                financial_value_kes=float(call.get("args", {}).get("total_amount", 0.0)),
                raw_payload=call,
                requires_human_approval=True
            )
            
            return {
                "dry_run_preview": dry_run,
                "final_response_text": f"⚠️ Action Required: Please approve {dry_run.action_type} before proceeding."
            }
            
    # For READ operations, return the final synthesized LLM answer
    messages = state.get("messages", [])
    last_content = messages[-1].content if messages else "Operation completed successfully."
    
    return {
        "dry_run_preview": None,
        "final_response_text": last_content
    }
