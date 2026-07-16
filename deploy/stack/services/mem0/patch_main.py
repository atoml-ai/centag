#!/usr/bin/env python3
"""Patch mem0 server/main.py for Ollama provider and dynamic embedding dims."""
import os

with open("/app/server/main.py", "r", encoding="utf-8") as f:
    content = f.read()

content = content.replace(
    'BUNDLED_LLM_PROVIDERS = ("openai", "anthropic", "gemini")',
    'BUNDLED_LLM_PROVIDERS = ("openai", "anthropic", "gemini", "ollama")',
)
content = content.replace(
    'BUNDLED_EMBEDDER_PROVIDERS = ("openai", "gemini")',
    'BUNDLED_EMBEDDER_PROVIDERS = ("openai", "gemini", "ollama")',
)

if "LLM_PROVIDER = os.environ.get" not in content:
    ollama_env_block = """OPENAI_API_KEY = os.environ.get("OPENAI_API_KEY")
LLM_PROVIDER = os.environ.get("LLM_PROVIDER", "openai")
EMBEDDER_PROVIDER = os.environ.get("EMBEDDER_PROVIDER", "openai")
OLLAMA_BASE_URL = os.environ.get("OLLAMA_BASE_URL", "http://localhost:11434")"""
    content = content.replace(
        'OPENAI_API_KEY = os.environ.get("OPENAI_API_KEY")',
        ollama_env_block,
    )

old_llm_block = """    "llm": {
        "provider": "openai",
        "config": {"api_key": OPENAI_API_KEY, "temperature": 0.2, "model": DEFAULT_LLM_MODEL},
    },
    "embedder": {"provider": "openai", "config": {"api_key": OPENAI_API_KEY, "model": DEFAULT_EMBEDDER_MODEL}},"""

if old_llm_block in content:
    new_llm_block = """    "llm": {
        "provider": LLM_PROVIDER,
        "config": (
            {"model": DEFAULT_LLM_MODEL, "temperature": 0.2, "ollama_base_url": OLLAMA_BASE_URL}
            if LLM_PROVIDER == "ollama"
            else {"api_key": OPENAI_API_KEY, "temperature": 0.2, "model": DEFAULT_LLM_MODEL}
        ),
    },
    "embedder": {
        "provider": EMBEDDER_PROVIDER,
        "config": (
            {"model": DEFAULT_EMBEDDER_MODEL, "ollama_base_url": OLLAMA_BASE_URL, "embedding_dims": int(os.environ.get("EMBEDDING_MODEL_DIMS", "768"))}
            if EMBEDDER_PROVIDER == "ollama"
            else {"api_key": OPENAI_API_KEY, "model": DEFAULT_EMBEDDER_MODEL}
        ),
    },"""
    content = content.replace(old_llm_block, new_llm_block)

if '"embedding_model_dims":' not in content:
    content = content.replace(
        '"collection_name": POSTGRES_COLLECTION_NAME,',
        '"collection_name": POSTGRES_COLLECTION_NAME,\n            "embedding_model_dims": int(os.environ.get("EMBEDDING_MODEL_DIMS", "1536")),',
    )

with open("/app/server/main.py", "w", encoding="utf-8") as f:
    f.write(content)

print("Patched /app/server/main.py successfully")