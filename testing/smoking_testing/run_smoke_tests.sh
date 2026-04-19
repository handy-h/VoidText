#!/bin/bash

# 小说文本清洗工具 - 冒烟测试脚本
# 用法: ./run_smoke_tests.sh [服务地址]

set -e

# 配置
SERVER_URL="${1:-http://localhost:8080}"
TEST_DATA_DIR="$(cd "$(dirname "$0")/test_data" && pwd)"
SAMPLE_FILE="$TEST_DATA_DIR/sample.txt"
TEST_MD5=""
TEST_FILE_MD5=""

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# 测试统计
TOTAL=0
PASSED=0
FAILED=0

# 打印函数
print_header() {
  echo ""
  echo "========================================="
  echo "  小说文本清洗工具 - 冒烟测试"
  echo "========================================="
  echo "测试时间: $(date '+%Y-%m-%d %H:%M:%S')"
  echo "服务地址: $SERVER_URL"
  echo ""
}

print_result() {
  local test_id=$1
  local test_name=$2
  local result=$3

  TOTAL=$((TOTAL + 1))

  if [ "$result" = "PASS" ]; then
    PASSED=$((PASSED + 1))
    printf "[${GREEN}PASS${NC}] "
  else
    FAILED=$((FAILED + 1))
    printf "[${RED}FAIL${NC}] "
  fi

  printf "%-8s %s\n" "[$test_id]" "$test_name"
}

check_service() {
  echo ""
  echo ">>> 检测服务状态..."

  if curl -s --connect-timeout 5 "$SERVER_URL" > /dev/null 2>&1; then
    echo "服务运行正常"
    return 0
  else
    echo -e "${RED}错误: 服务不可用 ($SERVER_URL)${NC}"
    echo "请确保服务已启动: go run ./cmd/txtclean/"
    exit 1
  fi
}

# 测试: 服务可用性
test_service_root() {
  local response
  response=$(curl -s "$SERVER_URL/")

  if echo "$response" | grep -q "html"; then
    print_result "SMOKE-001" "服务可用性" "PASS"
    return 0
  else
    print_result "SMOKE-001" "服务可用性" "FAIL"
    return 1
  fi
}

# 测试: API根路径
test_api_root() {
  local response
  response=$(curl -s "$SERVER_URL/api/files")

  if echo "$response" | grep -q '"success"'; then
    print_result "SMOKE-002" "API根路径" "PASS"
    return 0
  else
    print_result "SMOKE-002" "API根路径" "FAIL"
    return 1
  fi
}

# 测试: 静态资源
test_static_resource() {
  local response
  response=$(curl -s -o /dev/null -w "%{http_code}" "$SERVER_URL/static/css/style.css")

  if [ "$response" = "200" ]; then
    print_result "SMOKE-003" "静态资源访问" "PASS"
    return 0
  else
    print_result "SMOKE-003" "静态资源访问" "FAIL"
    return 1
  fi
}

# 测试: 上传测试文件
test_upload_file() {
  if [ ! -f "$SAMPLE_FILE" ]; then
    print_result "SMOKE-101" "上传测试文件" "FAIL"
    echo "  测试文件不存在: $SAMPLE_FILE"
    return 1
  fi

  local response
  response=$(curl -s -X POST "$SERVER_URL/api/files/upload" \
    -F "file=@$SAMPLE_FILE")

  if echo "$response" | grep -q '"success":true'; then
    TEST_MD5=$(echo "$response" | grep -o '"md5":"[^"]*"' | cut -d'"' -f4)
    print_result "SMOKE-101" "上传测试文件" "PASS"
    echo "  文件MD5: $TEST_MD5"
    return 0
  else
    print_result "SMOKE-101" "上传测试文件" "FAIL"
    echo "  响应: $response"
    return 1
  fi
}

# 测试: 列出文件
test_list_files() {
  local response
  response=$(curl -s "$SERVER_URL/api/files")

  if echo "$response" | grep -q "$TEST_MD5"; then
    print_result "SMOKE-102" "列出文件" "PASS"
    return 0
  else
    print_result "SMOKE-102" "列出文件" "FAIL"
    return 1
  fi
}

# 测试: 获取文件详情
test_get_file() {
  local response
  response=$(curl -s "$SERVER_URL/api/files/$TEST_MD5")

  if echo "$response" | grep -q '"md5"'; then
    print_result "SMOKE-103" "获取文件详情" "PASS"
    return 0
  else
    print_result "SMOKE-103" "获取文件详情" "FAIL"
    return 1
  fi
}

# 测试: 获取文件内容
test_get_file_content() {
  local response
  response=$(curl -s "$SERVER_URL/api/files/$TEST_MD5/content")

  if echo "$response" | grep -q '"success":true'; then
    print_result "SMOKE-104" "获取文件内容" "PASS"
    return 0
  else
    print_result "SMOKE-104" "获取文件内容" "FAIL"
    return 1
  fi
}

# 测试: 启动处理
test_start_processing() {
  local response
  response=$(curl -s -X POST "$SERVER_URL/api/files/$TEST_MD5/run")

  if echo "$response" | grep -q '"success":true'; then
    print_result "SMOKE-201" "启动处理" "PASS"
    return 0
  else
    print_result "SMOKE-201" "启动处理" "FAIL"
    return 1
  fi
}

# 测试: 获取处理状态
test_get_status() {
  sleep 2

  local response
  response=$(curl -s "$SERVER_URL/api/files/$TEST_MD5/status")

  if echo "$response" | grep -q '"status"'; then
    print_result "SMOKE-202" "获取处理状态" "PASS"
    return 0
  else
    print_result "SMOKE-202" "获取处理状态" "FAIL"
    return 1
  fi
}

# 测试: 获取审核项
test_get_review_items() {
  local response
  response=$(curl -s "$SERVER_URL/api/files/$TEST_MD5/review-items")

  if echo "$response" | grep -q '"success":true'; then
    print_result "SMOKE-203" "获取审核项" "PASS"
    return 0
  else
    print_result "SMOKE-203" "获取审核项" "FAIL"
    return 1
  fi
}

# 测试: 获取文件规则
test_get_rules() {
  local response
  response=$(curl -s "$SERVER_URL/api/files/$TEST_MD5/rules")

  if echo "$response" | grep -q '"success":true'; then
    print_result "SMOKE-401" "获取文件规则" "PASS"
    return 0
  else
    print_result "SMOKE-401" "获取文件规则" "FAIL"
    return 1
  fi
}

# 测试: 更新文件规则
test_update_rules() {
  local response
  response=$(curl -s -X PUT "$SERVER_URL/api/files/$TEST_MD5/rules" \
    -H "Content-Type: application/json" \
    -d '{"enableBasicCleaning":true,"enableVectorDetection":false}')

  if echo "$response" | grep -q '"success":true'; then
    print_result "SMOKE-402" "更新文件规则" "PASS"
    return 0
  else
    print_result "SMOKE-402" "更新文件规则" "FAIL"
    return 1
  fi
}

# 测试: 恢复文件处理
test_resume_file() {
  local response
  response=$(curl -s -X POST "$SERVER_URL/api/files/$TEST_MD5/resume")

  if echo "$response" | grep -q '"success"'; then
    print_result "SMOKE-204" "恢复文件处理" "PASS"
    return 0
  else
    print_result "SMOKE-204" "恢复文件处理" "FAIL"
    return 1
  fi
}

# 测试: 删除文件
test_delete_file() {
  local response
  response=$(curl -s -X DELETE "$SERVER_URL/api/files/$TEST_MD5")

  if echo "$response" | grep -q '"success":true'; then
    print_result "SMOKE-601" "删除文件" "PASS"
    return 0
  else
    print_result "SMOKE-601" "删除文件" "FAIL"
    return 1
  fi
}

# 打印汇总
print_summary() {
  echo ""
  echo "========================================="
  echo "  测试结果汇总"
  echo "========================================="
  echo "总用例数: $TOTAL"
  echo -e "通过: ${GREEN}$PASSED${NC}"
  echo -e "失败: ${RED}$FAILED${NC}"

  local rate=0
  if [ $TOTAL -gt 0 ]; then
    rate=$((PASSED * 100 / TOTAL))
  fi
  echo "通过率: $rate%"

  if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}所有测试通过!${NC}"
  else
    echo -e "${RED}有测试失败，请检查上述输出${NC}"
  fi
  echo "========================================="
}

# 主流程
main() {
  print_header

  check_service

  echo ""
  echo ">>> 执行冒烟测试..."

  # 基础服务测试
  test_service_root
  test_api_root
  test_static_resource

  # 文件操作测试
  test_upload_file
  test_list_files
  test_get_file
  test_get_file_content

  # 处理流程测试
  test_start_processing
  test_get_status
  test_get_review_items
  test_resume_file

  # 规则管理测试
  test_get_rules
  test_update_rules

  # 清理
  echo ""
  echo ">>> 清理测试数据..."
  test_delete_file

  print_summary

  if [ $FAILED -gt 0 ]; then
    exit 1
  fi
}

main "$@"