# Build stage - compile Go binary and Playwright CLI
FROM golang:1.25-bookworm AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o playwright-mcp-server ./cmd/server && \
    go build -o playwright-cli github.com/playwright-community/playwright-go/cmd/playwright

# Runtime stage - minimal image with Playwright browsers
FROM debian:bookworm-slim

# Install ca-certificates, Playwright browsers, and their system dependencies.
# The playwright-cli binary (built from the project's pinned playwright-go version)
# ensures browser versions match the library expectations.
COPY --from=builder /build/playwright-cli /usr/local/bin/playwright
RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates && \
    playwright install --with-deps && \
    rm /usr/local/bin/playwright && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Create non-root user and copy Playwright driver + browsers to its home
RUN useradd -r -m -s /usr/sbin/nologin mcpuser && \
    mkdir -p /home/mcpuser/.cache && \
    cp -r /root/.cache/ms-playwright /root/.cache/ms-playwright-go /home/mcpuser/.cache/ && \
    chown -R mcpuser:mcpuser /home/mcpuser/.cache

WORKDIR /app

# Copy server binary and default config
COPY --from=builder /build/playwright-mcp-server .
COPY config.yaml .
RUN chown -R mcpuser:mcpuser /app

USER mcpuser

# Override at runtime with -e MCP_CONFIG_PATH=/custom/config.yaml
ENV MCP_CONFIG_PATH=/app/config.yaml
# Bind to all interfaces so the server is accessible outside the container
ENV MCP_HOST=0.0.0.0

EXPOSE 3000

CMD ["./playwright-mcp-server"]
