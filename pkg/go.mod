module github.com/octelium/cordium/pkg

go 1.25.7

require (
	github.com/octelium/octelium/apis v0.0.0-00010101000000-000000000000
	github.com/octelium/octelium/pkg v0.0.0-20260303211601-19e614b75eab
	github.com/pkg/errors v0.9.1
	google.golang.org/protobuf v1.36.11
)

require (
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 // indirect
	google.golang.org/grpc v1.79.1 // indirect
)

replace github.com/octelium/octelium/apis => ../apis
