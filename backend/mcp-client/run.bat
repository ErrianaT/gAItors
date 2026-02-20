@ECHO OFF
SET APP_ROOT=""
SET KUBE_MCP_URL=http://localhost:9090/mcp
SET RTS_MCP_URL=http://localhost:9091/mcp
SET FILE_SEARCH_MCP_URL=http://localhost:9092/mcp
SET UF_MCP_URL=http://localhost:9093/mcp
SET LLM_URL=https://api.openai.com/v1/chat/completions
SET LLM_TOKEN=<token>

ECHO ============================================
ECHO   Starting mcp client (golang) 
ECHO   Time: %DATE% %TIME%
ECHO ============================================
ECHO.

.\main.exe