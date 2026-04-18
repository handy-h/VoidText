#!/bin/bash

echo "=== 启动文本清洗工具 ==="

echo "1. 检查Go依赖"
go mod tidy
go mod verify

echo "2. 构建项目"
go build -o txtclean ./cmd/txtclean

echo "3. 运行项目"
./txtclean
