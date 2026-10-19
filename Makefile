# aide —— 运行在本机的个人 AI 助理
# 常用目标：make build / test / test-race / cover / lint / serve / dist

APP      := aide
MODULE   := github.com/yixinzhangagent/aide

# 版本信息（构建期通过 -ldflags -X 注入）。
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT     ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X $(MODULE)/internal/version.Version=$(VERSION) \
	-X $(MODULE)/internal/version.Commit=$(COMMIT) \
	-X $(MODULE)/internal/version.BuildTime=$(BUILD_TIME)

# 固定 golangci-lint 版本，保证本地与 CI 结果一致。
# 需使用支持 Go 1.27 的较新版本。
GOLANGCI_VERSION := v2.15.0

.PHONY: all build run install test test-race cover test-integration fmt vet tidy lint serve dist clean

all: build

## build: 编译本机二进制到 bin/
build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(APP) ./cmd/aide

## run: 直接 go run（不注入版本）
run:
	go run ./cmd/aide

## install: 安装到 GOPATH/bin
install:
	go install -ldflags "$(LDFLAGS)" ./cmd/aide

## test: 运行全部单元测试
test:
	go test -count=1 ./...

## test-race: 开启竞态检测
test-race:
	go test -race -count=1 ./...

## cover: 生成覆盖率报告
cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

## test-integration: 真实调用本地 Ollama（需先启动服务）
test-integration:
	go test -tags=integration -count=1 -v ./test/integration/...

## fmt / vet / tidy: 基础整理
fmt:
	go fmt ./...

vet:
	go vet ./...

tidy:
	go mod tidy

## lint: 运行 golangci-lint（首次会按固定版本拉取）
lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_VERSION) run ./...

## serve: 启动本地 HTTP 服务
serve: build
	./bin/$(APP) serve

## dist: 交叉编译 darwin/linux 的 arm64/amd64
dist:
	@mkdir -p dist
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(APP)-darwin-arm64 ./cmd/aide
	GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(APP)-darwin-amd64 ./cmd/aide
	GOOS=linux  GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o dist/$(APP)-linux-arm64  ./cmd/aide
	GOOS=linux  GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/$(APP)-linux-amd64  ./cmd/aide
	@echo "已生成 dist/ 下四个平台二进制"

## clean: 清理构建产物
clean:
	rm -rf bin dist coverage.out
