.PHONY: build build-all clean test run fmt vet help

BINARY_NAME=clawdbot-bridge
VERSION?=0.1.0
BUILD_DIR=dist
SRC_DIR=cmd/bridge

help: ## 显示帮助信息
	@echo "可用的命令:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

build: ## 编译当前平台的二进制文件
	@echo "编译 $(BINARY_NAME)..."
	go build -o $(BINARY_NAME) ./$(SRC_DIR)/
	@echo "完成: $(BINARY_NAME)"

build-all: ## 编译所有平台的二进制文件
	@echo "编译所有平台..."
	./scripts/build.sh

clean: ## 清理构建文件
	@echo "清理..."
	rm -f $(BINARY_NAME)
	rm -rf $(BUILD_DIR)
	go clean
	@echo "清理完成"

test: ## 运行测试
	@echo "运行测试..."
	go test -v ./...

fmt: ## 格式化代码
	@echo "格式化代码..."
	go fmt ./...
	@echo "格式化完成"

vet: ## 检查代码
	@echo "检查代码..."
	go vet ./...
	@echo "检查完成"

lint: fmt vet ## 运行所有代码检查

tidy: ## 整理依赖
	@echo "整理依赖..."
	go mod tidy
	@echo "完成"

run: build ## 编译并运行
	@echo "运行 $(BINARY_NAME)..."
	./$(BINARY_NAME)

install: build ## 安装到 GOPATH/bin
	@echo "安装 $(BINARY_NAME)..."
	go install ./$(SRC_DIR)/
	@echo "已安装到: $$(go env GOPATH)/bin/$(BINARY_NAME)"

dev: ## 开发模式运行（不编译）
	@echo "开发模式运行..."
	go run ./$(SRC_DIR)/

deps: ## 下载依赖
	@echo "下载依赖..."
	go mod download
	@echo "完成"

.DEFAULT_GOAL := help
