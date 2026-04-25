package main

import (
	"fmt"
	"io/ioutil"
	
	"voidtext/internal/processor/preprocess"
)

func main() {
	// 测试清洗文件（包含大量替换字符）
	cleaningFilePath := "data/uploads/6f32ed28546fb02ec39873abace37cce_cleaning.txt"
	content, err := ioutil.ReadFile(cleaningFilePath)
	if err != nil {
		fmt.Printf("读取文件失败: %v\n", err)
		return
	}
	
	fmt.Println("=== 测试清洗文件（包含大量替换字符）===")
	fmt.Printf("原始文件大小: %d 字节\n", len(content))
	
	// 使用 PreprocessBytes 处理
	result, err := preprocess.PreprocessBytes(content)
	if err != nil {
		fmt.Printf("预处理失败: %v\n", err)
		return
	}
	
	fmt.Printf("预处理后长度: %d 字符\n", len(result.Content))
	
	// 统计替换字符数量
	replacementCount := 0
	for _, r := range result.Content {
		if r == '\uFFFD' {
			replacementCount++
		}
	}
	fmt.Printf("剩余替换字符数量: %d\n", replacementCount)
	
	// 统计乱码清理变更
	garbledChanges := 0
	for _, change := range result.Changes {
		if change.Type == "garbled_text_removal" {
			garbledChanges++
		}
	}
	fmt.Printf("乱码清理变更数量: %d\n", garbledChanges)
	
	// 显示前200个字符
	if len(result.Content) > 200 {
		fmt.Printf("前200字符: %q\n", result.Content[:200])
	}
	
	// 显示所有变更
	fmt.Println("\n变更记录:")
	for i, change := range result.Changes {
		if i < 10 { // 只显示前10个变更
			fmt.Printf("  变更%d: 类型=%s", i+1, change.Type)
			if change.Type == "garbled_text_removal" {
				fmt.Printf(", 原始长度=%d, 替换=%q", len(change.Original), change.Replacement)
			}
			fmt.Println()
		}
	}
	if len(result.Changes) > 10 {
		fmt.Printf("  ... 还有 %d 个变更\n", len(result.Changes)-10)
	}
}