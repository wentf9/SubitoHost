# 项目基本信息
BINARY_NAME=subitohost
MAIN_FILE=cmd/subitohost/main.go
BUILD_DIR=bin
VERSION?=0.1.0

# 构建参数
GO=go
CGO_ENABLED=0
GOFLAGS=-ldflags="-X main.Version=$(VERSION)"

.PHONY: all build clean test test-race test-short package install fmt help build-ui

all: build

## help: 显示此帮助信息
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^## [a-zA-Z_-]+:' Makefile | sed 's/## //g' | awk -F: '{printf "  %-15s %s\n", $$1, $$2}'

## build-ui: 编译前端产物
build-ui:
	@echo "正在编译前端界面..."
	cd ui && npm install && npm run build
	@echo "前端界面编译完成。"

## build: 编译二进制文件到 bin 目录
build: fmt build-ui
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_FILE)
	@echo "构建完成: $(BUILD_DIR)/$(BINARY_NAME)"

## clean: 清理构建产物
clean:
	rm -rf $(BUILD_DIR)
	@echo "清理完成。"

## test: 运行所有测试
test:
	CGO_ENABLED=$(CGO_ENABLED) $(GO) test ./... -v

## test-race: 运行带有并发竞争检测的测试
test-race:
	CGO_ENABLED=$(CGO_ENABLED) $(GO) test -race ./... -v

## test-short: 运行快速单元测试 (跳过硬件相关的测试)
test-short:
	CGO_ENABLED=$(CGO_ENABLED) $(GO) test -short ./... -v

## fmt: 格式化 Go 代码
fmt:
	$(GO) fmt ./...

## package: 将二进制文件、配置和歌单打包为 tar.gz
package: build
	@echo "正在打包..."
	@mkdir -p $(BUILD_DIR)/$(BINARY_NAME)-$(VERSION)
	@cp $(BUILD_DIR)/$(BINARY_NAME) $(BUILD_DIR)/$(BINARY_NAME)-$(VERSION)/
	@cp -r configs $(BUILD_DIR)/$(BINARY_NAME)-$(VERSION)/
	@cp -r setlists $(BUILD_DIR)/$(BINARY_NAME)-$(VERSION)/
	@tar -czf $(BUILD_DIR)/$(BINARY_NAME)-$(VERSION).tar.gz -C $(BUILD_DIR) $(BINARY_NAME)-$(VERSION)
	@rm -rf $(BUILD_DIR)/$(BINARY_NAME)-$(VERSION)
	@echo "打包完成: $(BUILD_DIR)/$(BINARY_NAME)-$(VERSION).tar.gz"

## install: 将二进制文件安装到 /usr/local/bin
install: build
	sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/$(BINARY_NAME)
	@echo "已安装到 /usr/local/bin/$(BINARY_NAME)"
