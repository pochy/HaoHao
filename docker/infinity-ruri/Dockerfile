FROM michaelf34/infinity:latest

RUN /app/.venv/bin/pip install --no-cache-dir --upgrade \
    "sentence-transformers>=3.4.1,<4" \
    "transformers>=4.48.0,<5" \
    "huggingface-hub>=0.27.0,<1" \
  && /app/.venv/bin/pip uninstall -y flash-attn || true
