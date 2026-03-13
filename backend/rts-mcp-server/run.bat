@ECHO OFF
SETLOCAL ENABLEDELAYEDEXPANSION

REM --- Environment variables ---
SET LLM_URL=https://api.openai.com/v1/chat/completions
SET RGRTA_API_KEY=<rgrta_api_key>
SET TRANSIT_API_KEY=<transit_api_key>
SET LLM_TOKEN=<llm_token>

ECHO ============================================
ECHO   Starting RTS MCP Server
ECHO   Time: %DATE% %TIME%
ECHO ============================================
ECHO.

REM --- Run server and show ALL logs in console ---
.\rts-mcp-server.exe -addr localhost:9091

