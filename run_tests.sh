#!/bin/bash

# 湮文 VoidText 测试运行脚本
# 运行所有单元测试

set -e

echo "=== 开始运行湮文 VoidText 测试 ==="
echo ""

# 检查是否安装了go test
if ! command -v go test &> /dev/null; then
    echo "错误: 未找到 go test 命令"
    exit 1
fi

# 设置测试环境变量
export DATA_DIR="./test_data"
export BASE_DIR="."

# 清理测试数据目录
if [ -d "$DATA_DIR" ]; then
    echo "清理测试数据目录: $DATA_DIR"
    rm -rf "$DATA_DIR"
fi

# 创建测试数据目录
mkdir -p "$DATA_DIR"

# 运行数据库测试
echo ""
echo "=== 运行数据库测试 ==="
cd internal/database/test
go test -v -count=1 ./...
cd ../../..

# 运行错误处理测试
echo ""
echo "=== 运行错误处理测试 ==="
cd internal/errors/test
go test -v -count=1 ./...
cd ../../..

# 运行中间件测试
echo ""
echo "=== 运行中间件测试 ==="
cd web/backend/middleware/test
go test -v -count=1 ./...
cd ../../../..

# 运行处理器测试
echo ""
echo "=== 运行处理器测试 ==="
cd web/backend/handlers/test
go test -v -count=1 ./...
cd ../../../..

# 运行所有测试
echo ""
echo "=== 运行所有测试 ==="
go test ./... -v -count=1

# 清理测试数据
echo ""
echo "=== 清理测试数据 ==="
if [ -d "$DATA_DIR" ]; then
    rm -rf "$DATA_DIR"
    echo "测试数据目录已清理"
fi

echo ""
echo "=== 测试完成 ==="
echo "所有测试通过！"