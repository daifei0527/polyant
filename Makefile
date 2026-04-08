# AgentWiki Makefile
# 构建系统配置

# 版本信息
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "v0.1.0-dev")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

# Go 相关
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOCLEAN := $(GOCMD) clean
GOFMT := $(GOCMD) fmt
GOVET := $(GOCMD) vet

# 目录
BUILD_DIR := ./bin
CMD_DIR := ./cmd

# 目标二进制
AGENTWIKI_BIN := $(BUILD_DIR)/agentwiki
AWCTL_BIN := $(BUILD_DIR)/awctl

# 交叉编译目标
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

.PHONY: all build clean test run init build-linux build-darwin build-windows cross-compile fmt vet lint help

# 默认目标：编译所有二进制
all: build

## build: 编译 agentwiki 和 awctl 二进制
build:
	@echo ">>> 编译 AgentWiki..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(AGENTWIKI_BIN) $(CMD_DIR)/agentwiki/
	$(GOBUILD) $(LDFLAGS) -o $(AWCTL_BIN) $(CMD_DIR)/awctl/
	@echo ">>> 编译完成: $(AGENTWIKI_BIN), $(AWCTL_BIN)"

## clean: 清除编译产物
clean:
	@echo ">>> 清理编译产物..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	@echo ">>> 清理完成"

## test: 运行测试
test:
	@echo ">>> 运行测试..."
	$(GOTEST) -v -race ./...
	@echo ">>> 测试完成"

## run: 编译并运行 agentwiki
run: build
	@echo ">>> 启动 AgentWiki..."
	./$(AGENTWIKI_BIN)

## init: 初始化项目（创建配置和数据目录）
init:
	@echo ">>> 初始化项目..."
	./$(AWCTL_BIN) init || $(GOBUILD) -o $(AWCTL_BIN) $(CMD_DIR)/awctl/ && ./$(AWCTL_BIN) init

## build-linux: 交叉编译 Linux 版本
build-linux:
	@echo ">>> 编译 Linux 版本..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/agentwiki-linux-amd64 $(CMD_DIR)/agentwiki/
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/awctl-linux-amd64 $(CMD_DIR)/awctl/
	@echo ">>> Linux 编译完成"

## build-darwin: 交叉编译 macOS 版本
build-darwin:
	@echo ">>> 编译 macOS 版本..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/agentwiki-darwin-amd64 $(CMD_DIR)/agentwiki/
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/awctl-darwin-amd64 $(CMD_DIR)/awctl/
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/agentwiki-darwin-arm64 $(CMD_DIR)/agentwiki/
	@echo ">>> macOS 编译完成"

## build-windows: 交叉编译 Windows 版本
build-windows:
	@echo ">>> 编译 Windows 版本..."
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/agentwiki-windows-amd64.exe $(CMD_DIR)/agentwiki/
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/awctl-windows-amd64.exe $(CMD_DIR)/awctl/
	@echo ">>> Windows 编译完成"

## cross-compile: 交叉编译所有平台
cross-compile:
	@echo ">>> 交叉编译所有平台..."
	@mkdir -p $(BUILD_DIR)
	@for p in $(PLATFORMS); do \
		GOOS=$${p%/*}; GOARCH=$${p#*/}; \
		out=$(BUILD_DIR)/agentwiki-$${GOOS}-$${GOARCH}; \
		[ "$${GOOS}" = "windows" ] && out=$${out}.exe; \
		echo "  编译 $$out..."; \
		GOOS=$${GOOS} GOARCH=$${GOARCH} $(GOBUILD) $(LDFLAGS) -o $$out $(CMD_DIR)/agentwiki/; \
		out=$(BUILD_DIR)/awctl-$${GOOS}-$${GOARCH}; \
		[ "$${GOOS}" = "windows" ] && out=$${out}.exe; \
		GOOS=$${GOOS} GOARCH=$${GOARCH} $(GOBUILD) $(LDFLAGS) -o $$out $(CMD_DIR)/awctl/; \
	done
	@echo ">>> 交叉编译完成"

## fmt: 格式化代码
fmt:
	@echo ">>> 格式化代码..."
	$(GOFMT) -s -w .
	@echo ">>> 格式化完成"

## vet: 静态分析
vet:
	@echo ">>> 静态分析..."
	$(GOVET) ./...
	@echo ">>> 静态分析完成"

## lint: 代码检查（fmt + vet）
lint: fmt vet

## help: 显示帮助信息
help:
	@echo "AgentWiki 构建系统"
	@echo ""
	@echo "使用方法:"
	@echo "  make <target>"
	@echo ""
	@echo "可用目标:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'
