#!/bin/bash

# 小说文本清洗工具 - 单元测试脚本
# 用法: ./run_unit_tests.sh [模块名]

set -e

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_ROOT"

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# 测试统计
TOTAL=0
PASSED=0
FAILED=0

print_header() {
  echo ""
  echo "========================================="
  echo "  小说文本清洗工具 - 单元测试"
  echo "========================================="
  echo "测试时间: $(date '+%Y-%m-%d %H:%M:%S')"
  echo "项目路径: $PROJECT_ROOT"
  echo ""
}

print_result() {
  local result=$1
  local package=$2
  local duration=$3

  TOTAL=$((TOTAL + 1))

  if [ "$result" = "PASS" ]; then
    PASSED=$((PASSED + 1))
    printf "[${GREEN}PASS${NC}] "
  else
    FAILED=$((FAILED + 1))
    printf "[${RED}FAIL${NC}] "
  fi

  printf "%-50s %s\n" "$package" "$duration"
}

# 检查是否有测试文件
check_tests() {
  echo ""
  echo ">>> 检查测试文件..."

  local test_count=$(find "$PROJECT_ROOT" -name "*_test.go" 2>/dev/null | wc -l)

  if [ "$test_count" -eq 0 ]; then
    echo -e "${YELLOW}警告: 未找到测试文件${NC}"
    echo "单元测试将在首次编写测试代码后生效"
    echo ""
    echo "示例 - 创建测试文件:"
    echo "  $PROJECT_ROOT/internal/file/md5_test.go"
    echo ""
    echo "测试函数示例:"
    echo '  func TestCalculateMD5(t *testing.T) {'
    echo '    result := CalculateMD5("hello")'
    echo '    expected := "5eb63bbbe01eeed093cb22bb8f5acdc3"'
    echo '    if result != expected {'
    echo '      t.Errorf("MD5 mismatch: got %s, want %s", result, expected)'
    echo '    }'
    echo '  }'
    return 1
  else
    echo "找到 $test_count 个测试文件"
    return 0
  fi
}

# 运行所有测试
run_all_tests() {
  echo ""
  echo ">>> 执行所有单元测试..."

  local output
  local status

  output=$(go test -v -count=1 ./... 2>&1) || status=$?

  if [ -z "$status" ] || [ "$status" = "0" ]; then
    print_result "PASS" "所有模块" ""
    echo "$output"
  else
    print_result "FAIL" "部分测试失败" ""
    echo "$output"
  fi
}

# 运行指定模块测试
run_module_tests() {
  local module=$1

  echo ""
  echo ">>> 执行模块测试: $module"

  local output
  local status

  output=$(go test -v -count=1 "./$module" 2>&1) || status=$?

  if [ -z "$status" ] || [ "$status" = "0" ]; then
    print_result "PASS" "$module" ""
  else
    print_result "FAIL" "$module" ""
    echo "$output"
  fi
}

# 生成覆盖率报告
run_coverage() {
  echo ""
  echo ">>> 生成覆盖率报告..."

  go test -coverprofile=coverage.out ./... 2>&1
  go tool cover -html=coverage.out -o coverage.html 2>&1

  echo "覆盖率报告已生成: coverage.html"
}

# 打印汇总
print_summary() {
  echo ""
  echo "========================================="
  echo "  测试结果汇总"
  echo "========================================="
  echo "总模块数: $TOTAL"
  echo -e "通过: ${GREEN}$PASSED${NC}"
  echo -e "失败: ${RED}$FAILED${NC}"

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

  local module="$1"

  if check_tests; then
    if [ -n "$module" ]; then
      run_module_tests "$module"
    else
      run_all_tests
    fi
  fi

  print_summary

  if [ $FAILED -gt 0 ]; then
    exit 1
  fi
}

main "$@"