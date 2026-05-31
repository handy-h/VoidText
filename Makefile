.PHONY: build dev run stop clean rebuild run-background help test fmt vet

.NOTPARALLEL:

BINARY_NAME = voidtext
MAIN_PACKAGE = ./cmd/voidtext/

# build : 编译二进制
build:
	@echo "========================================"
	@echo "  VoidText - Build"
	@echo "========================================"
	@echo ""
	@echo "[1/2] Checking Go dependencies..."
	@go mod tidy
	@go mod verify
	@echo ""
	@echo "[2/2] Compiling..."
	@go build -ldflags "-s -w" -o $(BINARY_NAME) $(MAIN_PACKAGE)
	@echo ""
	@echo "  Done."
	@echo "  Output: ./$(BINARY_NAME)"
	@echo ""
	@echo "  Next steps:"
	@echo "    make dev    - Run in development mode"
	@echo "    make run    - Run compiled binary"
	@echo "========================================"

# dev : 开发模式（控制台日志 + Gin debug）
dev:
	@echo "========================================"
	@echo "  VoidText - Development Mode"
	@echo "========================================"
	@echo ""
	@echo "  Features:"
	@echo "    - Console logging"
	@echo "    - Gin debug routes"
	@echo "    - go run (no compile needed)"
	@echo ""
	@echo "  Stop: Ctrl+C"
	@echo "========================================"
	@echo ""
	@LOG_TO_CONSOLE=true GIN_MODE=debug go run $(MAIN_PACKAGE)

# run : 运行已编译的二进制（生产环境）
run:
	@echo "========================================"
	@echo "  VoidText - Production Mode"
	@echo "========================================"
	@echo ""
ifeq ($(OS),Windows_NT)
	@if not exist $(BINARY_NAME).exe ( \
		echo "  Binary not found. Run: make build" \
		&& exit /b 1 \
	)
else
	@if [ ! -f $(BINARY_NAME) ]; then \
		echo "  Binary not found. Run: make build"; \
		exit 1; \
	fi
endif
	@echo "  Config : .env"
	@echo "  Data   : DATA_DIR (default: ./data)"
	@echo "  URL    : http://localhost:$(or $(PORT),8080)"
	@echo "========================================"
	@echo ""
ifeq ($(OS),Windows_NT)
	@$(BINARY_NAME).exe
else
	@./$(BINARY_NAME)
endif

# stop : 停止运行中的进程
stop:
	@echo "========================================"
	@echo "  VoidText - Stop"
	@echo "========================================"
ifeq ($(OS),Windows_NT)
	@-taskkill /F /IM $(BINARY_NAME).exe 2>nul && echo "  Stopped" || echo "  No running process found"
else
	@-pkill -f "$(BINARY_NAME)" 2>/dev/null && echo "  Stopped" || echo "  No running process found"
endif
	@echo ""

# clean : 删除编译产物（保留数据）
clean: stop
	@echo "========================================"
	@echo "  VoidText - Clean"
	@echo "========================================"
	@echo ""
ifeq ($(OS),Windows_NT)
	@if exist $(BINARY_NAME).exe (del /F /Q $(BINARY_NAME).exe && echo "  Removed: $(BINARY_NAME).exe")
else
	@rm -f $(BINARY_NAME)
	@echo "  Removed: $(BINARY_NAME)"
endif
	@echo ""
	@echo "  Kept: source, .env, data/"
	@echo "    - database (cleaning.db)"
	@echo "    - uploads/"
	@echo "    - backups/"
	@echo "    - temp/"
	@echo "========================================"

# rebuild : 停止 + 清理 + 重新编译
rebuild:
	@$(MAKE) stop
	@$(MAKE) clean
	@$(MAKE) build

# run-background : 后台运行（不阻塞终端）
run-background:
	@echo "========================================"
	@echo "  VoidText - Background Mode"
	@echo "========================================"
	@echo ""
ifeq ($(OS),Windows_NT)
	@if not exist $(BINARY_NAME).exe ( \
		echo "  Binary not found. Run: make build" \
		&& exit /b 1 \
	)
	@start /B $(BINARY_NAME).exe >nul 2>&1
	@echo "  Started in background"
else
	@if [ ! -f $(BINARY_NAME) ]; then \
		echo "  Binary not found. Run: make build"; \
		exit 1; \
	fi
	@nohup ./$(BINARY_NAME) > /dev/null 2>&1 &
	@echo "  Started in background (PID: $$!)"
endif
	@echo "  URL: http://localhost:$(or $(PORT),8080)"
	@echo "  Stop: make stop"
	@echo "========================================"
	@echo ""

# test : 运行全部测试
test:
	@echo "Running tests..."
	@go test ./...

# fmt : 格式化代码
fmt:
	@echo "Formatting..."
	@go fmt ./...

# vet : 静态检查
vet:
	@echo "Running go vet..."
	@go vet ./...

# help : 显示帮助
help:
	@echo "========================================"
	@echo "  VoidText - Available Commands"
	@echo "========================================"
	@echo ""
	@echo "  make build          Compile binary"
	@echo "  make dev            Development mode (console logs)"
	@echo "  make run            Run compiled binary"
	@echo "  make stop           Stop running process"
	@echo "  make clean          Remove binary (keep data)"
	@echo "  make rebuild        Stop + clean + build"
	@echo "  make run-background Run in background"
	@echo "  make test           Run all tests"
	@echo "  make fmt            Format code"
	@echo "  make vet            Static analysis"
	@echo "  make help           Show this help"
	@echo ""
	@echo "========================================"
