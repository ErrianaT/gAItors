@ECHO OFF
SET GOOS=windows
SET GOARCH=amd64
go build -o uf-mcp-server.exe uf-mcp-server.go