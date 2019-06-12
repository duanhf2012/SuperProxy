SET CGO_ENABLED=0
SET GOOS=windows
SET GOARCH=amd64
set GOPATH=%~dp0/../../
go build  -v -o OriginServer.exe