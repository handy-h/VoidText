.PHONY: build dev run stop clean rebuild run-background help

.NOTPARALLEL:

BINARY_NAME = voidtext
MAIN_PACKAGE = ./cmd/voidtext/

## build   : 编译二进制文件
build:
	@echo "========================================"
	@echo "  湮文 VoidText — 编译项目"
	@echo "========================================"
	@echo ""
	@echo "[1/2] 检查 Go 依赖..."
	@go mod tidy
	@go mod verify
	@echo ""
	@echo "[2/2] 编译二进制文件..."
	go build -ldflags "-s -w" -o $(BINARY_NAME) $(MAIN_PACKAGE)
	@echo ""
	@echo "========================================"
	@echo "  ✓ 编译成功"
	@echo "  输出: ./$(BINARY_NAME)"
	@ls -lh $(BINARY_NAME) | awk '{print "  大小: " $$5}'
	@echo ""
	@echo "  运行方式:"
	@echo "    make dev    — 开发者模式（控制台日志）"
	@echo "    make run    — 生产环境运行"
	@echo "========================================"

## dev     : 开发者模式运行，控制台打印日志，支持热重载
dev:
	@echo "========================================"
	@echo "  湮文 VoidText — 开发者模式"
	@echo "========================================"
	@echo ""
	@echo "  功能说明:"
	@echo "  • 控制台实时打印日志 — 便于排查问题"
	@echo "  • Gin debug 模式 — 显示路由和请求详情"
	@echo "  • go run 直接运行 — 无需提前编译"
	@echo ""
	@echo "  停止方式: Ctrl+C"
	@echo "========================================"
	@echo ""
	LOG_TO_CONSOLE=true GIN_MODE=debug go run $(MAIN_PACKAGE)

## run     : 运行已编译的二进制文件（生产环境）
run:
	@echo "========================================"
	@echo "  湮文 VoidText — 生产模式启动"
	@echo "========================================"
	@echo ""
ifneq ($(wildcard $(BINARY_NAME)),)
	@echo "  ✓ 检测到编译产物: ./$(BINARY_NAME)"
else
	@echo "  ✗ 未检测到编译产物，请先执行 make build"
	@echo ""
	@false
endif
	@echo ""
	@echo "  启动说明:"
	@echo "  • 配置文件: .env（从可执行文件目录或工作目录加载）"
	@echo "  • 数据目录: DATA_DIR（默认 ./data）"
	@echo "  • 后台运行示例: nohup ./$(BINARY_NAME) > /dev/null 2>&1 &"
	@echo ""
	@echo "  访问地址: http://localhost:$(or $(PORT),8080)"
	@echo "========================================"
	@echo ""
	./$(BINARY_NAME)

## stop    : 停止运行中的进程
stop:
	@echo "========================================"
	@echo "  湮文 VoidText — 停止进程"
	@echo "========================================"
	@echo ""
	@-pkill "$(BINARY_NAME)" 2>/dev/null && echo "  ✓ 已停止进程" || echo "  - 未发现运行中的进程"
	@echo ""

## clean   : 删除编译产物（保留数据库、上传文件、备份等数据）
clean: stop
	@echo "========================================"
	@echo "  湮文 VoidText — 清理编译产物"
	@echo "========================================"
	@echo ""
	@rm -f $(BINARY_NAME)
	@echo "  ✓ 已删除: ./$(BINARY_NAME)"
	@echo ""
	@echo "========================================"
	@echo "  ✓ 清理完毕"
	@echo "  已删除: 编译产物"
	@echo "  保留: 源代码、配置文件 (.env)、数据目录 (data/):"
	@echo "    • 数据库 (cleaning.db)"
	@echo "    • 上传文件 (uploads/)"
	@echo "    • 备份文件 (backups/)"
	@echo "    • 临时文件 (temp/)"
	@echo "========================================"

## rebuild : 停止进程 → 删除编译产物 → 重新编译 (stop + clean + build)
.PHONY: rebuild
rebuild:
	@$(MAKE) stop
	@$(MAKE) clean
	@$(MAKE) build

## run-background : 后台运行（不阻塞终端，日志写入文件）
run-background:
	@echo "========================================"
	@echo "  湮文 VoidText — 后台模式启动"
	@echo "========================================"
	@echo ""
ifneq ($(wildcard $(BINARY_NAME)),)
	@echo "  ✓ 检测到编译产物: ./$(BINARY_NAME)"
else
	@echo "  ✗ 未检测到编译产物，请先执行 make build"
	@echo ""
	@false
endif
	@echo "  启动说明:"
	@echo "  • 配置文件: .env（从工作目录加载）"
	@echo "  • 数据目录: DATA_DIR（默认 ./data）"
	@echo "  • 日志: 重定向到 /dev/null"
	@echo ""
	@nohup ./$(BINARY_NAME) > /dev/null 2>&1 & echo "  ✓ 已后台启动" && echo "  进程 PID: $$!" && echo "  访问地址: http://localhost:$(or $(PORT),8080)"
	@echo "  停止方式: make stop"
	@echo "========================================"
	@echo ""

## help    : 显示帮助信息
help:
	@echo "========================================"
	@echo "  湮文 VoidText — 项目管理"
	@echo "========================================"
	@echo ""
	@echo "  用法:"
	@echo ""
	@echo "  make build    编译二进制文件"
	@echo "               执行 go mod tidy/verify + go build"
	@echo ""
	@echo "  make dev      开发者模式运行"
	@echo "               控制台打印日志, Gin debug 模式, go run 热重载"
	@echo ""
	@echo "  make run      生产环境运行"
	@echo "               运行已编译的 ./$(BINARY_NAME) 二进制文件"
	@echo ""
	@echo "  make stop     停止运行中的进程 (pkill)"
	@echo ""
	@echo "  make clean    删除编译产物"
	@echo "               保留 数据库、上传文件、备份等数据"
	@echo ""
	@echo "  make rebuild  停止 + 清理 + 重新编译"
	@echo ""
	@echo "  make run-background  后台运行（不阻塞终端）"
	@echo ""
	@echo "  make help     显示此帮助"
	@echo ""
	@echo "========================================"
