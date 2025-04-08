# 设置 Go 编译环境变量
export GO111MODULE=on
export GOPROXY=https://goproxy.cn,direct

# 设置版本信息
VERSION := $(shell git describe --tags --always --dirty)
BUILDTIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILDTIME)"

# 设置输出目录
BUILD_DIR := build
BINARY_NAME := dumpall-go

# 默认目标
.PHONY: all
all: clean build-all

# 清理构建目录
.PHONY: clean
clean:
	@echo "Cleaning build directory..."
	@rm -rf $(BUILD_DIR)
	@mkdir -p $(BUILD_DIR)

# 构建所有平台
.PHONY: build-all
build-all: build-windows build-linux build-darwin

# 构建 Windows 版本
.PHONY: build-windows
build-windows:
	@echo "Building for Windows..."
	@GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe
	@GOOS=windows GOARCH=386 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-386.exe

# 构建 Linux 版本
.PHONY: build-linux
build-linux:
	@echo "Building for Linux..."
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64
	@GOOS=linux GOARCH=386 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-386
	@GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64

# 构建 macOS 版本
.PHONY: build-darwin
build-darwin:
	@echo "Building for macOS..."
	@GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64
	@GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64

# 构建当前平台
.PHONY: build
build:
	@echo "Building for current platform..."
	@go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)

# 运行测试
.PHONY: test
test:
	@echo "Running tests..."
	@go test -v ./...

# 安装依赖
.PHONY: deps
deps:
	@echo "Installing dependencies..."
	@go mod tidy
	@go mod download

# 帮助信息
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  all          - Build for all platforms"
	@echo "  build        - Build for current platform"
	@echo "  build-windows- Build for Windows (amd64, 386)"
	@echo "  build-linux  - Build for Linux (amd64, 386, arm64)"
	@echo "  build-darwin - Build for macOS (amd64, arm64)"
	@echo "  clean        - Clean build directory"
	@echo "  test         - Run tests"
	@echo "  deps         - Install dependencies"
	@echo "  help         - Show this help message" 