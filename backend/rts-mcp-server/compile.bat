@ECHO OFF
SET GOOS=windows
SET GOARCH=amd64
go build -o rts-mcp-server.exe rts-mcp-server.go