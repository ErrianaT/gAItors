import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { HttpServerTransport } from "@modelcontextprotocol/sdk/http.js";
import http from "http";
import { tools } from "./tools.js";

const server = new McpServer({
  name: "file-search-store-extension",
  version: "1.0.1",
});

for (const { name, schema, func } of tools) {
  server.registerTool(name, schema, func);
}

const PORT = 8000;
const HOST = "localhost";

const transport = new HttpServerTransport();
const httpServer = http.createServer(transport.handler);

await server.connect(transport);

httpServer.listen(PORT, HOST, () => {
  console.log(`MCP Server running on http://${HOST}:${PORT}/mcp`);
});