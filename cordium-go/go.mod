module github.com/octelium/cordium/cordium-go

go 1.26.8

require (
	github.com/octelium/octelium/apis v0.42.0
	github.com/octelium/octelium/octelium-go v0.0.0-20260918205337-11f813eee6c3
	google.golang.org/grpc v1.83.2
	google.golang.org/protobuf v1.36.12
)

require (
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/oauth2 v0.36.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260526163538-3dc84a4a5aaa // indirect
)

replace github.com/octelium/octelium/apis => ../apis
