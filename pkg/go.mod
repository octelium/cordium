module github.com/octelium/cordium/pkg

go 1.26.6

require (
	github.com/octelium/octelium/apis v0.0.0-00010101000000-000000000000
	github.com/octelium/octelium/pkg v0.0.0-20260912095159-4af4f27d984c
	github.com/pkg/errors v0.9.1
	google.golang.org/protobuf v1.36.12
)

require (
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260526163538-3dc84a4a5aaa // indirect
	google.golang.org/grpc v1.83.2 // indirect
)

replace github.com/octelium/octelium/apis => ../apis
