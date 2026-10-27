FROM python:3.14-slim

WORKDIR /app

# Install uv securely
COPY --from=ghcr.io/astral-sh/uv:latest /uv /bin/uv

# Replicate the project directory
COPY pyproject.toml uv.lock ./
# Pre-install dependencies
RUN uv sync --frozen --no-dev

COPY src/ ./src/

EXPOSE 8000
EXPOSE 50051

CMD [".venv/bin/fastapi", "run", "src/main.py", "--port", "8000"]
