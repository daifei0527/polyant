# Polyant Makefile
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
SEED_BIN := $(BUILD_DIR)/seed
USER_BIN := $(BUILD_DIR)/user
PACTL_BIN := $(BUILD_DIR)/pactl

# 交叉编译目标
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

.PHONY: all build build-seed build-user clean test init build-linux build-darwin build-windows cross-compile fmt vet lint help docker-seed docker-user build-admin dev-admin

# 默认目标：编译所有二进制
all: build

## build: 编译 seed, user 和 pactl 二进制
build:
	@echo ">>> 编译 Polyant..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(SEED_BIN) $(CMD_DIR)/seed/
	$(GOBUILD) $(LDFLAGS) -o $(USER_BIN) $(CMD_DIR)/user/
	$(GOBUILD) $(LDFLAGS) -o $(PACTL_BIN) $(CMD_DIR)/pactl/
	@echo ">>> 编译完成: $(SEED_BIN), $(USER_BIN), $(PACTL_BIN)"

## build-seed: 仅编译种子节点二进制
build-seed:
	@echo ">>> 编译种子节点..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(SEED_BIN) $(CMD_DIR)/seed/
	@echo ">>> 编译完成: $(SEED_BIN)"

## build-user: 仅编译用户节点二进制
build-user:
	@echo ">>> 编译用户节点..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(USER_BIN) $(CMD_DIR)/user/
	@echo ">>> 编译完成: $(USER_BIN)"

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

## init: 初始化项目（创建配置和数据目录）
init:
	@echo ">>> 初始化项目..."
	./$(PACTL_BIN) init || $(GOBUILD) -o $(PACTL_BIN) $(CMD_DIR)/pactl/ && ./$(PACTL_BIN) init

## build-linux: 交叉编译 Linux 版本
build-linux:
	@echo ">>> 编译 Linux 版本..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/seed-linux-amd64 $(CMD_DIR)/seed/
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/user-linux-amd64 $(CMD_DIR)/user/
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/pactl-linux-amd64 $(CMD_DIR)/pactl/
	@echo ">>> Linux 编译完成"

## build-darwin: 交叉编译 macOS 版本
build-darwin:
	@echo ">>> 编译 macOS 版本..."
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/seed-darwin-amd64 $(CMD_DIR)/seed/
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/user-darwin-amd64 $(CMD_DIR)/user/
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/pactl-darwin-amd64 $(CMD_DIR)/pactl/
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/seed-darwin-arm64 $(CMD_DIR)/seed/
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/user-darwin-arm64 $(CMD_DIR)/user/
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/pactl-darwin-arm64 $(CMD_DIR)/pactl/
	@echo ">>> macOS 编译完成"

## build-windows: 交叉编译 Windows 版本
build-windows:
	@echo ">>> 编译 Windows 版本..."
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/seed-windows-amd64.exe $(CMD_DIR)/seed/
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/user-windows-amd64.exe $(CMD_DIR)/user/
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/pactl-windows-amd64.exe $(CMD_DIR)/pactl/
	@echo ">>> Windows 编译完成"

## cross-compile: 交叉编译所有平台
cross-compile:
	@echo ">>> 交叉编译所有平台..."
	@mkdir -p $(BUILD_DIR)
	@for p in $(PLATFORMS); do \
		GOOS=$${p%/*}; GOARCH=$${p#*/}; \
		for cmd in seed user pactl; do \
			out=$(BUILD_DIR)/$${cmd}-$${GOOS}-$${GOARCH}; \
			[ "$${GOOS}" = "windows" ] && out=$${out}.exe; \
			echo "  编译 $$out..."; \
			CGO_ENABLED=0 GOOS=$${GOOS} GOARCH=$${GOARCH} $(GOBUILD) $(LDFLAGS) -o $$out $(CMD_DIR)/$${cmd}/; \
		done; \
	done
	@echo ">>> 交叉编译完成"

## docker-seed: 构建种子节点 Docker 镜像
docker-seed:
	@echo ">>> 构建种子节点 Docker 镜像..."
	docker build -f Dockerfile.seed -t polyant-seed:$(VERSION) .
	docker tag polyant-seed:$(VERSION) polyant-seed:latest
	@echo ">>> Docker 镜像构建完成: polyant-seed:$(VERSION)"

## docker-user: 构建用户节点 Docker 镜像
docker-user:
	@echo ">>> 构建用户节点 Docker 镜像..."
	docker build -f Dockerfile.user -t polyant-user:$(VERSION) .
	docker tag polyant-user:$(VERSION) polyant-user:latest
	@echo ">>> Docker 镜像构建完成: polyant-user:$(VERSION)"

## build-admin: 构建管理页面前端
build-admin:
	@echo ">>> 构建管理页面..."
	cd web/admin && npm install && npm run build
	@echo ">>> 管理页面构建完成"

## dev-admin: 开发模式运行管理页面
dev-admin:
	@echo ">>> 启动管理页面开发服务器..."
	cd web/admin && npm run dev

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
	@echo "Polyant 构建系统"
	@echo ""
	@echo "使用方法:"
	@echo "  make <target>"
	@echo ""
	@echo "可用目标:"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'
