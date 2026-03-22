@ECHO OFF
echo UF Senior Project - One Stop Agentic AI Application

SET LLM_URL=https://api.openai.com/v1/chat/completions
SET LLM_TOKEN=<LLM_TOKEN>

@ECHO OFF

echo "RAG MCP Serevr Starting..."
SET GEMINI_API_KEY=<GEMINI_API_KEY>
SET FILE_LOCATION=C:\Downloads\senior-project-demo\gemini-file-search\one-stop
ECHO ============================================
ECHO   Starting RAG MCP server
ECHO   Time: %DATE% %TIME%
ECHO ============================================
ECHO.
REM Launch Node rag-mcp-server in a new terminal 
start cmd /k "title RAG MCP Server && cd /d C:\Downloads\senior-project-demo\rag-mcp-server && npx mcp-proxy --port 9092 --shell node src/file-search-store-extension.js"
REM Wait briefly to ensure server starts (optional) 
timeout /t 2 >nul

@ECHO OFF

echo "RTS MCP Serevr Starting..."
SET LLM_URL=https://api.openai.com/v1/chat/completions
SET RGRTA_API_KEY=<RGRTA_API_KEY>
SET TRANSIT_API_KEY=<TRANSIT_API_KEY>
SET LLM_TOKEN=<LLM_TOKEN>
ECHO ============================================
ECHO   Starting RTS MCP server
ECHO   Time: %DATE% %TIME%
ECHO ============================================
ECHO.
start cmd /k "title RTS MCP Server && cd /d C:\Downloads\senior-project-demo\rts-mcp-server && call C:\Downloads\senior-project-demo\rts-mcp-server\rts-mcp-server.exe -addr localhost:9091"
REM Wait briefly to ensure server starts (optional) 
timeout /t 2 >nul

@ECHO OFF

echo "UF MCP Serevr Starting..."
SET LLM_URL=https://api.openai.com/v1/chat/completions
SET RGRTA_API_KEY=<RGRTA_API_KEY>
SET TRANSIT_API_KEY=<TRANSIT_API_KEY>
SET ARCGIS_CLIENT_ID=Gg14ahVMjc7ZaexU
SET ARCGIS_CLIENT_SECRET=<ARCGIS_CLIENT_SECRET>
SET GEMINI_API_KEY=<GEMINI_API_KEY>
SET OPENWEATHERMAP_KEY=<OPENWEATHERMAP_KEY>
SET LLM_TOKEN=<LLM_TOKEN>
ECHO ============================================
ECHO   Starting UF MCP server
ECHO   Time: %DATE% %TIME%
ECHO ============================================
ECHO.
start cmd /k "title UF MCP Server && cd /d C:\Downloads\senior-project-demo\uf-mcp-server && call C:\Downloads\senior-project-demo\uf-mcp-server\uf-mcp-server.exe -addr localhost:9093"
REM Wait briefly to ensure server starts (optional) 
timeout /t 2 >nul

@ECHO OFF

echo "mcp client Starting..."
SET APP_ROOT=C:\Downloads\DIY\agentic-ai\mcp-client\AppRoot
SET KUBE_MCP_URL=http://localhost:9090/mcp
SET RTS_MCP_URL=http://localhost:9091/mcp
SET FILE_SEARCH_MCP_URL=http://localhost:9092/mcp
SET UF_MCP_URL=http://localhost:9093/mcp
SET LLM_URL=https://api.openai.com/v1/chat/completions
SET LLM_TOKEN=<LLM_TOKEN>
echo ... starting web chat client
start cmd /k "title mcp-client && cd /d C:\Downloads\senior-project-demo\mcp-client && call C:\Downloads\senior-project-demo\mcp-client\main.exe"
REM Wait briefly to ensure server starts (optional) 
timeout /t 2 >nul

