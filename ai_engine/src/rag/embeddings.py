from langchain_openai import OpenAIEmbeddings
from src.core.runtime_llm import snapshot

def get_embeddings():
    """Returns the unified embeddings model."""
    # This assumes Groq doesn't provide embeddings out of the box in the same way,
    # or leverages a standard compliant endpoint. If using groq strictly, 
    # we might use HuggingFaceEmbeddings instead. For now, OpenAI is used.
    return OpenAIEmbeddings(
        model="text-embedding-3-small", 
        api_key=snapshot().api_key or "sk-dummy"
    )
