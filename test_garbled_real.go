package main

import (
	"fmt"
	"io/ioutil"
	
	"voidtext/internal/processor/preprocess"
)

func main() {
	// 测试实际损坏的清洗文件
	cleaningFilePath := "data/uploads/6f32ed28546fb02ec39873abace37cce_cleaning.txt"
	cleaningContent, err := ioutil.ReadFile(cleaningFilePath)
	if err != nil {
		fmt.Printf("读取清洗文件失败: %v\n", err)
		return
	}
	
	fmt.Println("=== 测试实际损坏的清洗文件 ===")
	fmt.Printf("文件大小: %d 字节\n", len(cleaningContent))
	
	// 使用 PreprocessBytes 处理
	result, err := preprocess.PreprocessBytes(cleaningContent)
	if err != nil {
		fmt.Printf("预处理失败: %v\n", err)
		return
	}
	
	fmt.Printf("处理后长度: %d 字符\n", len(result.Content))
	
	// 统计替换字符数量
	replacementCount := 0
	for _, r := range result.Content {
		if r == '\uFFFD' {
			replacementCount++
		}
	}
	fmt.Printf("处理后替换字符数量: %d\n", replacementCount)
	
	// 统计乱码清理变更
	garbledChanges := 0
	for _, change := range result.Changes {
		if change.Type == "garbled_text_removal" {
			garbledChanges++
		}
	}
	fmt.Printf("乱码清理变更数量: %d\n", garbledChanges)
	
	// 显示前300个字符
	if len(result.Content) > 300 {
		fmt.Printf("前300字符: %q\n", result.Content[:300])
	} else {
		fmt.Printf("内容: %q\n", result.Content)
	}
	
	// 测试原始GBK文件
	originalFilePath := "data/uploads/6f32ed28546fb02ec39873abace37cce_于冒泡~恶搞暗黑破坏神.txt"
	originalContent, err := ioutil.ReadFile(originalFilePath)
	if err != nil {
		fmt.Printf("读取原始文件失败: %v\n", err)
		return
	}
	
	fmt.Println("\n=== 测试原始GBK文件 ===")
	fmt.Printf("文件大小: %d 字节\n", len(originalContent))
	
	result2, err := preprocess.PreprocessBytes(originalContent)
	if err != nil {
		fmt.Printf("预处理失败: %v\n", err)
		return
	}
	
	fmt.Printf("处理后长度: %d 字符\n", len(result2.Content))
	
	replacementCount2 := 0
	for _, r := range result2.Content {
		if r == '\uFFFD' {
			replacementCount2++
		}
	}
	fmt.Printf("处理后替换字符数量: %d\n", replacementCount2)
	
	garbledChanges2 := 0
	for _, change := range result2.Changes {
		if change.Type == "garbled_text_removal" {
			garbledChanges2++
		}
	}
	fmt.Printf("乱码清理变更数量: %d\n", garbledChanges2)
	
	if len(result2.Content) > 300 {
		fmt.Printf("前300字符: %q\n", result2.Content[:300])
	}
}