package preprocess

import (
	"strings"
	"testing"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// toGBK 将 UTF-8 字符串编码为 GBK 字节
func toGBK(t *testing.T, s string) []byte {
	t.Helper()
	// 使用 GBK 编码器将 UTF-8 转为 GBK 字节
	reader := transform.NewReader(strings.NewReader(s), simplifiedchinese.GBK.NewEncoder())
	buf := make([]byte, 0, len(s)*2)
	tmp := make([]byte, 1024)
	for {
		n, err := reader.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	return buf
}

// toGB18030 将 UTF-8 字符串编码为 GB18030 字节
func toGB18030(t *testing.T, s string) []byte {
	t.Helper()
	encoder := simplifiedchinese.GB18030.NewEncoder()
	reader := transform.NewReader(strings.NewReader(s), encoder)
	buf := make([]byte, 0, len(s)*2)
	tmp := make([]byte, 1024)
	for {
		n, err := reader.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	return buf
}

// TestShould_正确处理_纯UTF8文件 测试纯 UTF-8 文件直接返回
func TestShould_正确处理_纯UTF8文件(t *testing.T) {
	input := "这是一段纯UTF-8编码的中文文本。\n第二行内容。"
	data := []byte(input)

	result, err := detectAndConvertToUTF8(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != input {
		t.Errorf("纯UTF-8应原样返回\ngot:  %q\nwant: %q", result, input)
	}
}

// TestShould_正确处理_纯ASCII文件 测试纯 ASCII 文件（兼容 UTF-8）
func TestShould_正确处理_纯ASCII文件(t *testing.T) {
	input := "Hello, World!\nThis is a test."
	data := []byte(input)

	result, err := detectAndConvertToUTF8(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != input {
		t.Errorf("纯ASCII应原样返回\ngot:  %q\nwant: %q", result, input)
	}
}

// TestShould_正确处理_空文件 测试空文件
func TestShould_正确处理_空文件(t *testing.T) {
	data := []byte{}

	result, err := detectAndConvertToUTF8(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("空文件应返回空字符串\ngot: %q", result)
	}
}

// TestShould_正确转换_纯GBK文件 测试纯 GBK 编码文件
func TestShould_正确转换_纯GBK文件(t *testing.T) {
	expected := "这是一段GBK编码的中文文本。"
	data := toGBK(t, expected)

	// 确认 GBK 字节不是有效 UTF-8
	if utf8.Valid(data) {
		t.Fatal("测试数据应为非UTF-8编码")
	}

	result, err := detectAndConvertToUTF8(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !utf8.ValidString(result) {
		t.Errorf("结果应为有效UTF-8: %q", result)
	}
	if result != expected {
		t.Errorf("GBK转换结果不正确\ngot:  %q\nwant: %q", result, expected)
	}
}

// TestShould_正确转换_纯GB18030文件 测试纯 GB18030 编码文件
func TestShould_正确转换_纯GB18030文件(t *testing.T) {
	expected := "这是一段GB18030编码的中文文本。"
	data := toGB18030(t, expected)

	// 确认 GB18030 字节不是有效 UTF-8
	if utf8.Valid(data) {
		t.Fatal("测试数据应为非UTF-8编码")
	}

	result, err := detectAndConvertToUTF8(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !utf8.ValidString(result) {
		t.Errorf("结果应为有效UTF-8: %q", result)
	}
	if result != expected {
		t.Errorf("GB18030转换结果不正确\ngot:  %q\nwant: %q", result, expected)
	}
}

// TestShould_正确处理_混合编码文件_UTF8和GBK行 测试同一文件中部分行 UTF-8、部分行 GBK
func TestShould_正确处理_混合编码文件_UTF8和GBK行(t *testing.T) {
	utf8Line := "这是UTF-8编码的行。"
	gbkLine := "这是GBK编码的行。"
	gbkBytes := toGBK(t, gbkLine)

	// 构造混合编码：UTF-8行 + GBK行 + UTF-8行
	var mixed []byte
	mixed = append(mixed, []byte(utf8Line)...)
	mixed = append(mixed, '\n')
	mixed = append(mixed, gbkBytes...)
	mixed = append(mixed, '\n')
	mixed = append(mixed, []byte(utf8Line)...)

	// 确认混合字节不是有效 UTF-8
	if utf8.Valid(mixed) {
		t.Fatal("混合编码数据应为非UTF-8")
	}

	result, err := detectAndConvertToUTF8(mixed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !utf8.ValidString(result) {
		t.Errorf("结果应为有效UTF-8: %q", result)
	}

	lines := strings.Split(result, "\n")
	if len(lines) != 3 {
		t.Fatalf("应有3行，实际 %d 行", len(lines))
	}
	if lines[0] != utf8Line {
		t.Errorf("第1行不正确\ngot:  %q\nwant: %q", lines[0], utf8Line)
	}
	if lines[1] != gbkLine {
		t.Errorf("第2行(原GBK)不正确\ngot:  %q\nwant: %q", lines[1], gbkLine)
	}
	if lines[2] != utf8Line {
		t.Errorf("第3行不正确\ngot:  %q\nwant: %q", lines[2], utf8Line)
	}
}

// TestShould_正确处理_混合编码文件_多行GBK 测试多行连续 GBK 行
func TestShould_正确处理_混合编码文件_多行GBK(t *testing.T) {
	lines := []string{
		"第一行中文。",
		"第二行中文。",
		"第三行中文。",
	}
	var mixed []byte
	for i, line := range lines {
		if i%2 == 0 {
			// 偶数行用 GBK
			mixed = append(mixed, toGBK(t, line)...)
		} else {
			// 奇数行用 UTF-8
			mixed = append(mixed, []byte(line)...)
		}
		mixed = append(mixed, '\n')
	}

	result, err := detectAndConvertToUTF8(mixed)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !utf8.ValidString(result) {
		t.Errorf("结果应为有效UTF-8: %q", result)
	}

	resultLines := strings.Split(strings.TrimRight(result, "\n"), "\n")
	if len(resultLines) != 3 {
		t.Fatalf("应有3行，实际 %d 行", len(resultLines))
	}
	for i, line := range lines {
		if resultLines[i] != line {
			t.Errorf("第%d行不正确\ngot:  %q\nwant: %q", i+1, resultLines[i], line)
		}
	}
}

// TestShould_正确处理_PreprocessBytes_乱码清理 测试 PreprocessBytes 中的乱码清理
func TestShould_正确处理_PreprocessBytes_乱码清理(t *testing.T) {
	// 包含经典乱码模式
	input := "正常文本锟斤拷锟斤拷更多正常文本"
	data := []byte(input)

	result, err := PreprocessBytes(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 应该标记删除了乱码
	if !strings.Contains(result.Content, "因无法修复的乱码删除了") {
		t.Errorf("应标记乱码删除，实际内容: %q", result.Content)
	}
	// 应该保留正常文本
	if !strings.Contains(result.Content, "正常文本") {
		t.Errorf("应保留正常文本，实际内容: %q", result.Content)
	}
	if !strings.Contains(result.Content, "更多正常文本") {
		t.Errorf("应保留正常文本，实际内容: %q", result.Content)
	}
}

// TestShould_正确处理_PreprocessBytes_替换字符 测试包含 U+FFFD 替换字符的文本
func TestShould_正确处理_PreprocessBytes_替换字符(t *testing.T) {
	input := "正常文本���更多正常文本"
	data := []byte(input)

	result, err := PreprocessBytes(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 应该标记删除了连续替换字符
	if !strings.Contains(result.Content, "因无法修复的乱码删除了") {
		t.Errorf("应标记乱码删除，实际内容: %q", result.Content)
	}
}

// TestShould_正确处理_fixMixedEncoding_单行GBK 测试 fixMixedEncoding 对单行 GBK 的处理
func TestShould_正确处理_fixMixedEncoding_单行GBK(t *testing.T) {
	expected := "这是GBK行"
	gbkBytes := toGBK(t, expected)
	input := string(gbkBytes)

	result := fixMixedEncoding(input)
	if !utf8.ValidString(result) {
		t.Errorf("结果应为有效UTF-8: %q", result)
	}
	if result != expected {
		t.Errorf("转换结果不正确\ngot:  %q\nwant: %q", result, expected)
	}
}

// TestShould_正确处理_fixMixedEncoding_纯UTF8 测试 fixMixedEncoding 对纯 UTF-8 的无损处理
func TestShould_正确处理_fixMixedEncoding_纯UTF8(t *testing.T) {
	input := "这是纯UTF-8文本\n第二行\n第三行"
	result := fixMixedEncoding(input)
	if result != input {
		t.Errorf("纯UTF-8应原样返回\ngot:  %q\nwant: %q", result, input)
	}
}

// TestShould_正确处理_PreprocessBytes_完整流程 测试 PreprocessBytes 的完整预处理流程
func TestShould_正确处理_PreprocessBytes_完整流程(t *testing.T) {
	// GBK 编码的文本，包含广告和多余空白
	expected := "第一段正文内容。第二段正文内容。"
	gbkData := toGBK(t, expected)

	result, err := PreprocessBytes(gbkData)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !utf8.ValidString(result.Content) {
		t.Errorf("结果应为有效UTF-8: %q", result.Content)
	}
	// 验证内容正确转换
	if !strings.Contains(result.Content, "第一段正文内容") {
		t.Errorf("应包含正文内容，实际: %q", result.Content)
	}
}
