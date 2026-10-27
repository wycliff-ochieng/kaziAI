from src.rag.vector_store import get_vector_store

async def get_tenant_rag_context(tenant_id: str, query: str) -> str:
    """
    Given a user's prompt, retrieves company policies, ERP custom field structures, 
    or logic chains specific to that tenant from pgvector.
    """
    store = get_vector_store()
    
    # Filter strictly by tenant isolation
    docs = store.similarity_search_with_score(
        query=query,
        k=3,
        filter={"tenant_id": tenant_id}
    )
    
    if not docs:
        return "No specific business rules configured."
        
    contexts = []
    for doc, score in docs:
        if score > 0.7:  # Threshold
            contexts.append(doc.page_content)
    
    if not contexts:
        return "No specific business rules configured."
        
    return "\n\n".join(contexts)
