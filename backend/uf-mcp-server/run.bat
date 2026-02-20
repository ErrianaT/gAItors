@ECHO OFF
SET ARCGIS_CLIENT_ID=<client id>
SET ARCGIS_CLIENT_SECRET=<client secret>
SET OPENWEATHERMAP_KEY=<map key>

ECHO ============================================
ECHO   Starting UF MCP Server
ECHO   Time: %DATE% %TIME%
ECHO ============================================
ECHO.

.\uf-mcp-server.exe -addr localhost:9093
