#!/bin/bash

echo "=== 测试小说文本清洗工具 ==="

echo "1. 检查项目结构"
ls -la

echo "2. 检查Go依赖"
go mod tidy
go mod verify

echo "3. 构建项目"
go build -o txtclean ./cmd/txtclean

echo "4. 运行项目（后台）"
./txtclean &

# 等待服务启动
sleep 5

echo "5. 测试API响应"
curl -X GET http://localhost:8080/api/files

echo "6. 停止服务"
pkill -f txtclean

echo "=== 测试完成 ==="