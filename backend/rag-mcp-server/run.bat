@ECHO OFF
SET GEMINI_API_KEY=<GEMINI_API_KEY>
SET FILE_LOCATION=C:\Downloads\senior-project-demo\file-search\plc

ECHO ============================================
ECHO   Starting RAG MCP Server
ECHO   Time: %DATE% %TIME%
ECHO ============================================
ECHO.

npx mcp-proxy --port 9092 --shell "node src/file-search-store-extension.js"
