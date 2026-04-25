#!/bin/bash

# 树莓派启动脚本 - 后台运行 VoidText 服务
# Usage: ./raspberrypi-start.sh [--log-dir <dir>] [--stop] [--status]

# 默认配置
APP_NAME="voidtext"
APP_BINARY="./voidtext"
LOG_DIR="/mnt/ssd/voidtext/log"
PID_FILE="./pid.txt"

# 解析参数
while [[ $# -gt 0 ]]; do
  case $1 in
    --log-dir)
      LOG_DIR="$2"
      shift 2
      ;;
    --stop)
      ACTION="stop"
      shift
      ;;
    --status)
      ACTION="status"
      shift
      ;;
    *)
      ACTION="start"
      shift
      ;;
  esac

done

# 确保日志目录存在
mkdir -p "$LOG_DIR"

# 停止服务
if [ "$ACTION" = "stop" ]; then
  if [ -f "$PID_FILE" ]; then
    PID=$(cat "$PID_FILE")
    if ps -p "$PID" > /dev/null 2>&1; then
      echo "停止服务 (PID: $PID)..."
      kill "$PID"
      sleep 2
      if ps -p "$PID" > /dev/null 2>&1; then
        echo "强制停止服务..."
        kill -9 "$PID"
      fi
      rm -f "$PID_FILE"
      echo "服务已停止"
    else
      echo "服务未运行 (PID 文件存在但进程不存在)"
      rm -f "$PID_FILE"
    fi
  else
    echo "服务未运行 (未找到 PID 文件)"
  fi
  exit 0

# 查看状态
elif [ "$ACTION" = "status" ]; then
  if [ -f "$PID_FILE" ]; then
    PID=$(cat "$PID_FILE")
    if ps -p "$PID" > /dev/null 2>&1; then
      echo "服务正在运行 (PID: $PID)"
      echo "日志目录: $LOG_DIR"
    else
      echo "服务未运行 (PID 文件存在但进程不存在)"
      rm -f "$PID_FILE"
    fi
  else
    echo "服务未运行 (未找到 PID 文件)"
  fi
  exit 0

# 启动服务
else
  # 检查是否已运行
  if [ -f "$PID_FILE" ]; then
    PID=$(cat "$PID_FILE")
    if ps -p "$PID" > /dev/null 2>&1; then
      echo "服务已在运行 (PID: $PID)"
      exit 0
    else
      echo "清理无效的 PID 文件"
      rm -f "$PID_FILE"
    fi
  fi

  # 检查二进制文件
  if [ ! -f "$APP_BINARY" ]; then
    echo "错误: 未找到二进制文件 '$APP_BINARY'"
    echo "请先编译: go build -o voidtext ./cmd/voidtext/"
    exit 1
  fi

  # 启动服务到后台
  echo "启动服务..."
  echo "日志输出到: $LOG_DIR"
  
  # 后台运行并记录 PID
  "$APP_BINARY" > "$LOG_DIR/stdout.log" 2> "$LOG_DIR/stderr.log" &
  PID=$!
  
  # 验证服务是否启动成功
  sleep 2
  if ps -p "$PID" > /dev/null 2>&1; then
    echo "$PID" > "$PID_FILE"
    echo "服务已启动 (PID: $PID)"
    echo "访问地址: http://$(hostname -I | awk '{print $1}'):8080"
  else
    echo "服务启动失败，请查看日志: $LOG_DIR/stderr.log"
    exit 1
  fi
fi