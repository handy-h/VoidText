#!/bin/bash

echo "=== 启动湮文 VoidText ==="

echo "1. 检查Go依赖"
go mod tidy
go mod verify

echo "2. 构建项目"
go build -o voidtext ./cmd/voidtext

echo "3. 运行项目"
./voidtext
