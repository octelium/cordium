module github.com/octelium/cordium/client/cordium

go 1.25.7

require (
	github.com/fatih/color v1.18.0
	github.com/grpc-ecosystem/go-grpc-middleware v1.4.0
	github.com/octelium/cordium/pkg v0.0.0-00010101000000-000000000000
	github.com/octelium/octelium/apis v0.0.0-00010101000000-000000000000
	github.com/octelium/octelium/client/common v0.0.0-20260216185830-b4f621edcfe8
	github.com/octelium/octelium/pkg v0.0.0-20260216185830-b4f621edcfe8
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.10.2
	go.uber.org/zap v1.27.0
	golang.org/x/term v0.37.0
	google.golang.org/grpc v1.78.0
	google.golang.org/protobuf v1.36.11
)

require (
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2 // indirect
	github.com/chzyer/readline v0.0.0-20180603132655-2972be24d48e // indirect
	github.com/clipperhouse/displaywidth v0.6.0 // indirect
	github.com/clipperhouse/stringish v0.1.1 // indirect
	github.com/clipperhouse/uax29/v2 v2.3.0 // indirect
	github.com/denisbrodbeck/machineid v1.0.1 // indirect
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-resty/resty/v2 v2.17.1 // indirect
	github.com/gofrs/flock v0.13.0 // indirect
	github.com/google/go-attestation v0.6.0 // indirect
	github.com/google/go-tpm v0.9.6 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-version v1.8.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/manifoldco/promptui v0.9.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.19 // indirect
	github.com/octelium/octelium/octelium-go v0.0.0-20260216185830-b4f621edcfe8 // indirect
	github.com/olekukonko/cat v0.0.0-20250911104152-50322a0618f6 // indirect
	github.com/olekukonko/errors v1.1.0 // indirect
	github.com/olekukonko/ll v0.1.3 // indirect
	github.com/olekukonko/tablewriter v1.1.2 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	github.com/zcalusic/sysinfo v1.1.3 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/text v0.31.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251029180050-ab9386a59fda // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/octelium/octelium/apis => ../../apis

replace github.com/octelium/cordium/pkg => ../../pkg
