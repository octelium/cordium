
export GO111MODULE=on
export GOPRIVATE=github.com/octelium/*
export PATH := $(PATH):$(shell go env GOPATH)/bin

.PHONY: gen-api build-cli build-octelium clean fmt lint test unit vendor

REPOSITORY := github.com/octelium/octelium
REGISTRY ?= ghcr.io
IMAGE_PREFIX := octelium

COMMIT := $(shell git rev-parse HEAD)
TAG := $(shell git describe --exact-match $(COMMIT) 2>/dev/null)
BRANCH := $(shell git rev-parse --abbrev-ref HEAD)

LDFLAGS_PATH := $(REPOSITORY)/pkg/utils/ldflags

LDF_IMAGE_REGISTRY := $(LDFLAGS_PATH).ImageRegistry=$(REGISTRY)
LDF_IMAGE_REGISTRY_PREFIX := $(LDFLAGS_PATH).ImageRegistryPrefix=$(IMAGE_PREFIX)
LDF_COMMIT := $(LDFLAGS_PATH).GitCommit=$(COMMIT)
LDF_TAG := $(LDFLAGS_PATH).GitTag=$(TAG)
LDF_BRANCH := $(LDFLAGS_PATH).GitBranch=$(BRANCH)

GENERATED_API_DOCS_DIR := ./tmp/docs/apis
GENERATED_API_DOCS_TEMP := ./unsorted/protoc/template.tmpl
GO_BIN_DIR := $${HOME}/go/bin

LDFLAGS := -ldflags '-X $(LDF_COMMIT) -X $(LDF_TAG) -X $(LDF_BRANCH)\
-X $(LDF_IMAGE_REGISTRY) -X $(LDF_IMAGE_REGISTRY_PREFIX)'

PROTO_GO_OPT := --go_opt=paths=source_relative
PROTO_GO_OPT_GRPC := $(PROTO_GO_OPT) --go-grpc_opt=paths=source_relative
PROTO_IN_PREFIX := apis/protobuf
PROTO_IN_MAIN := $(PROTO_IN_PREFIX)/main
PROTO_IN_CLUSTER := $(PROTO_IN_PREFIX)/cluster
PROTO_IN_CLIENT := $(PROTO_IN_PREFIX)/client
PROTO_IN_RSC := $(PROTO_IN_PREFIX)/rsc

CMD_TIDY := go mod tidy

build-cli-cordium:
	CGO_ENABLED=0 go build $(LDFLAGS) -o bin/ github.com/octelium/cordium/client/cordium

build-workspace:
	CGO_ENABLED=0 GOOS=linux go build $(LDFLAGS) -o bin/cordium-workspace github.com/octelium/cordium/cluster/workspace
build-workspace-git-cred-helper:
	CGO_ENABLED=0 GOOS=linux go build $(LDFLAGS) -o bin/cordium-git-cred-helper github.com/octelium/cordium/cluster/workspace/gitcredhelper
build-portal:
	CGO_ENABLED=0 GOOS=linux go build $(LDFLAGS) -o bin/cordium-portal github.com/octelium/cordium/cluster/portal
build-nocturne:
	CGO_ENABLED=0 GOOS=linux go build $(LDFLAGS) -o bin/cordium-nocturne github.com/octelium/cordium/cluster/nocturne
build-supervisor:
	CGO_ENABLED=0 GOOS=linux go build $(LDFLAGS) -o bin/cordium-supervisor github.com/octelium/cordium/cluster/supervisor
build-apiserver:
	CGO_ENABLED=0 GOOS=linux go build $(LDFLAGS) -o bin/cordium-apiserver github.com/octelium/cordium/cluster/apiserver
build-genesis:
	CGO_ENABLED=0 GOOS=linux go build $(LDFLAGS) -o bin/cordium-genesis github.com/octelium/cordium/cluster/genesis
build-mockapiserver:
	CGO_ENABLED=0 GOOS=linux go build $(LDFLAGS) -o bin/cordium-mockapiserver github.com/octelium/cordium/cluster/mockapiserver
build-rscserver:
	CGO_ENABLED=0 GOOS=linux go build $(LDFLAGS) -o bin/cordium-rscserver github.com/octelium/cordium/cluster/rscserver
build-vigil:
	CGO_ENABLED=0 GOOS=linux go build $(LDFLAGS) -o bin/cordium-vigil github.com/octelium/cordium/cluster/vigil
vendor:
	go mod tidy
	go mod vendor


protoc-install:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.1
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
	go install github.com/pseudomuto/protoc-gen-doc/cmd/protoc-gen-doc@latest
	mkdir -p $(GENERATED_API_DOCS_DIR)


gen-go-main:
	mkdir -p apis/main/metav1 apis/main/corev1 apis/main/authv1 apis/main/userv1
	mkdir -p apis/main/cordiumv1 
	protoc -I . -I $(PROTO_IN_MAIN)/metav1 metav1.proto \
		--go_out=apis/main/metav1 --go-grpc_out=apis/main/metav1 $(PROTO_GO_OPT)
	protoc -I . -I $(PROTO_IN_MAIN)/corev1 corev1.proto \
		--go_out=apis/main/corev1 --go-grpc_out=apis/main/corev1 $(PROTO_GO_OPT_GRPC)
	protoc -I . -I $(PROTO_IN_MAIN)/userv1 userv1.proto \
		--go_out=apis/main/userv1 --go-grpc_out=apis/main/userv1 $(PROTO_GO_OPT_GRPC)
	protoc -I . -I $(PROTO_IN_MAIN)/authv1 authv1.proto \
		--go_out=apis/main/authv1 --go-grpc_out=apis/main/authv1 $(PROTO_GO_OPT_GRPC)
	
	protoc -I . -I $(PROTO_IN_MAIN)/cordiumv1 cordiumv1.proto \
		--go_out=apis/main/cordiumv1 --go-grpc_out=apis/main/cordiumv1 $(PROTO_GO_OPT_GRPC)

gen-go-cluster:
	mkdir -p apis/cluster/cclusterv1 apis/cluster/coctovigilv1 apis/cluster/cvigilv1
	mkdir -p apis/cluster/ccordiumv1
	mkdir -p apis/cluster/csecretmanv1
	protoc -I . -I $(PROTO_IN_CLUSTER)/clusterv1 cclusterv1.proto \
		--go_out=apis/cluster/cclusterv1 --go-grpc_out=apis/cluster/cclusterv1 $(PROTO_GO_OPT)
	protoc -I . -I $(PROTO_IN_CLUSTER)/octovigilv1 coctovigilv1.proto \
		--go_out=apis/cluster/coctovigilv1 --go-grpc_out=apis/cluster/coctovigilv1 $(PROTO_GO_OPT_GRPC)
	protoc -I . -I $(PROTO_IN_CLUSTER)/cordiumv1 ccordiumv1.proto \
		--go_out=apis/cluster/ccordiumv1 --go-grpc_out=apis/cluster/ccordiumv1 $(PROTO_GO_OPT_GRPC)
	protoc -I . -I $(PROTO_IN_CLUSTER)/secretmanv1 csecretmanv1.proto \
		--go_out=apis/cluster/csecretmanv1 --go-grpc_out=apis/cluster/csecretmanv1 $(PROTO_GO_OPT_GRPC)
	protoc -I . -I $(PROTO_IN_CLUSTER)/vigilv1 cvigilv1.proto \
		--go_out=apis/cluster/cvigilv1 --go-grpc_out=apis/cluster/cvigilv1 $(PROTO_GO_OPT_GRPC)

gen-go-rsc:
	mkdir -p apis/rsc/rmetav1 apis/rsc/rcorev1 apis/rsc/rcachev1 apis/rsc/rratelimitv1
	mkdir -p apis/rsc/rcordiumv1
	protoc -I . -I $(PROTO_IN_RSC)/metav1 rmetav1.proto \
		--go_out=apis/rsc/rmetav1 --go-grpc_out=apis/rsc/rmetav1 $(PROTO_GO_OPT)
	protoc -I . -I $(PROTO_IN_RSC)/corev1 rcorev1.proto \
		--go_out=apis/rsc/rcorev1 --go-grpc_out=apis/rsc/rcorev1 $(PROTO_GO_OPT_GRPC)

	protoc -I . -I $(PROTO_IN_RSC)/cordiumv1 rcordiumv1.proto \
		--go_out=apis/rsc/rcordiumv1 --go-grpc_out=apis/rsc/rcordiumv1 $(PROTO_GO_OPT_GRPC)
	protoc -I . -I $(PROTO_IN_RSC)/cachev1 rcachev1.proto \
		--go_out=apis/rsc/rcachev1 --go-grpc_out=apis/rsc/rcachev1 $(PROTO_GO_OPT_GRPC)
	protoc -I . -I $(PROTO_IN_RSC)/ratelimitv1 rratelimitv1.proto \
		--go_out=apis/rsc/rratelimitv1 --go-grpc_out=apis/rsc/rratelimitv1 $(PROTO_GO_OPT_GRPC)


gen-go-client:
	mkdir -p apis/client/cliconfigv1
	protoc -I . -I $(PROTO_IN_CLIENT)/configv1 configv1.proto \
		--go_out=apis/client/cliconfigv1 --go-grpc_out=apis/client/cliconfigv1 $(PROTO_GO_OPT)

gen-api: gen-go-main gen-go-cluster gen-go-client gen-go-rsc
	rm -rf ./apis/protobuf
	go run unsorted/licenser/main.go

tidy:
	cd apis; $(CMD_TIDY)
	cd pkg; $(CMD_TIDY)
	cd client/cordium; $(CMD_TIDY)
	cd cluster/common; $(CMD_TIDY)
	cd cluster/rscserver; $(CMD_TIDY)
	cd cluster/workspace; $(CMD_TIDY)
	cd cluster/supervisor; $(CMD_TIDY)
	cd cluster/apiserver; $(CMD_TIDY)
	cd cluster/genesis; $(CMD_TIDY)
	cd cluster/nocturne; $(CMD_TIDY)
	cd cluster/portal; $(CMD_TIDY)
	cd cluster/mockapiserver; $(CMD_TIDY)
	cd cluster/vigil; $(CMD_TIDY)