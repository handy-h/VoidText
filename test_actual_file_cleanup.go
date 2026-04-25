package main

import (
	"fmt"
	"io/ioutil"
	"strings"
	
	"voidtext/internal/processor/preprocess"
)

func main() {
	// 测试实际文件
	filePath := "data/uploads/6f32ed28546fb02ec39873abace37cce_cleaning.txt"
	contentBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		fmt.Printf("读取文件失败: %v\n", err)
		return
	}
	
	fmt.Printf("文件大小: %d 字节\n", len(contentBytes))
	
	// 使用 PreprocessBytes 处理
	result, err := preprocess.PreprocessBytes(contentBytes)
	if err != nil {
		fmt.Printf("预处理失败: %v\n", err)
		return
	}
	
	fmt.Printf("预处理后长度: %d 字符\n", len(result.Content))
	
	// 统计替换字符
	replacementCountBefore := strings.Count(string(contentBytes), "�")
	replacementCountAfter := strings.Count(result.Content, "�")
	fmt.Printf("处理前替换字符数量: %d\n", replacementCountBefore)
	fmt.Printf("处理后替换字符数量: %d\n", replacementCountAfter)
	
	// 显示前200个字符
	if len(result.Content) > 200 {
		fmt.Printf("\n前200字符:\n%q\n", result.Content[:200])
	}
	
	// 显示变更
	fmt.Printf("\n变更数量: %d\n", len(result.Changes))
	
	// 统计不同类型的变更
	garbledRemovals := 0
	encodingFixes := 0
	otherChanges := 0
	
	for _, change := range result.Changes {
		switch change.Type {
		case "garbled_text_removal":
			garbledRemovals++
		case "encoding_fix":
			encodingFixes++
		default:
			otherChanges++
		}
	}
	
	fmt.Printf("乱码清理变更: %d\n", garbledRemovals)
	fmt.Printf("编码修复变更: %d\n", encodingFixes)
	fmt.Printf("其他变更: %d\n", otherChanges)
	
	// 显示前5个乱码清理变更
	fmt.Println("\n前5个乱码清理变更:")
	count := 0
	for _, change := range result.Changes {
		if change.Type == "garbled_text_removal" && count < 5 {
			fmt.Printf("  位置: %d, 原始长度: %d, 替换: %q\n", 
				change.Position, len(change.Original), change.Replacement)
			count++
		}
	}
	
	// 测试原始文件
	fmt.Println("\n=== 测试原始文件 ===")
	originalFilePath := "data/uploads/6f32ed28546fb02ec39873abace37cce_于冒泡~恶搞暗黑破坏神.txt"
	originalBytes, err := ioutil.ReadFile(originalFilePath)
	if err != nil {
		fmt.Printf("读取原始文件失败: %v\n", err)
		return
	}
	
	originalResult, err := preprocess.PreprocessBytes(originalBytes)
	if err != nil {
		fmt.Printf("预处理原始文件失败: %v\n", err)
		return
	}
	
	fmt.Printf("原始文件预处理后长度: %d 字符\n", len(originalResult.Content))
	originalReplacementCount := strings.Count(originalResult.Content, "�")
	fmt.Printf("原始文件替换字符数量: %d\n", originalReplacementCount)
	
	if len(originalResult.Content) > 200 {
		fmt.Printf("\n原始文件前200字符:\n%q\n", originalResult.Content[:200])
	}
	
	// 比较两个结果
	fmt.Println("\n=== 比较清洗文件和原始文件 ===")
	if len(result.Content) > 100 && len(originalResult.Content) > 100 {
		cleaningSample := result.Content[:100]
		originalSample := originalResult.Content[:100]
		
		fmt.Printf("清洗文件样本: %q\n", cleaningSample)
		fmt.Printf("原始文件样本: %q\n", originalSample)
		
		// 检查是否相同
		if cleaningSample == originalSample {
			fmt.Println("前100字符相同")
		} else {
			fmt.Println("前100字符不同")
			
			// 找出差异
			for i := 0; i < len(cleaningSample) && i < len(originalSample); i++ {
				if cleaningSample[i] != originalSample[i] {
					fmt.Printf("第一个差异在位置 %d: 清洗='%c'(%U), 原始='%c'(%U)\n", 
						i, cleaningSample[i], cleaningSample[i], originalSample[i], originalSample[i])
					break
				}
			}
		}
	}
}