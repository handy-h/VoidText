package main

import (
	"fmt"
	"log"
	"time"

	"voidtext/internal/config"
	"voidtext/internal/external"
)

func main() {
	// 加载 .env 配置（在项目根目录运行）
	if err := config.Load(); err != nil {
		log.Fatalf("配置加载失败: %v", err)
	}

	fmt.Println("============================================")
	fmt.Println("  湮文 VoidText — LLM修复性能对比测试")
	fmt.Println("============================================")
	fmt.Printf("本地模型: %s (%s)\n", config.AppConfigInstance.LocalModelName, config.AppConfigInstance.LocalModelURL)
	fmt.Printf("远程模型: %s (%s)\n", config.AppConfigInstance.CompletionModelName, config.AppConfigInstance.LLMApiURL)
	fmt.Println()

	// 从清洗文件中截取约 1500 字的小说段落作为测试样本
	sampleText := extractSample()

	fmt.Printf("测试文本长度: %d 字符\n", len([]rune(sampleText)))
	fmt.Println("--- 测试文本预览 ---")
	fmt.Println(sampleText[:100] + "...")
	fmt.Println()

	// ========== 1. 本地 Ollama 测试 ==========
	fmt.Println("━━━ [1/2] 本地 Ollama 模型修复 ━━━")
	ollamaClient := external.NewOllamaClient(
		config.AppConfigInstance.LocalModelURL,
		config.AppConfigInstance.LocalModelName,
		time.Duration(config.AppConfigInstance.LocalModelTimeout)*time.Second,
	)

	startLocal := time.Now()
	localResult, localErr := ollamaClient.CorrectText(sampleText)
	localDuration := time.Since(startLocal)

	if localErr != nil {
		fmt.Printf("❌ 本地模型失败: %v\n", localErr)
	} else {
		localRunes := len([]rune(localResult))
		ratio := float64(localRunes) / float64(len([]rune(sampleText))) * 100
		fmt.Printf("✅ 耗时: %v (%.1f 秒)\n", localDuration, localDuration.Seconds())
		fmt.Printf("   输入: %d 字 → 输出: %d 字 (%.0f%%)\n", len([]rune(sampleText)), localRunes, ratio)
		if localResult == sampleText {
			fmt.Println("   输出与原文本完全相同 (未做修改)")
		} else {
			fmt.Printf("   输出预览: %s...\n", truncate(localResult, 80))
		}
	}
	fmt.Println()

	// ========== 2. 远程 API 测试 ==========
	fmt.Println("━━━ [2/2] 远程 API 模型修复 ━━━")
	apiClient := external.NewAPI()

	startRemote := time.Now()
	remoteResult, remoteErr := apiClient.CorrectText(sampleText)
	remoteDuration := time.Since(startRemote)

	if remoteErr != nil {
		fmt.Printf("❌ 远程API失败: %v\n", remoteErr)
	} else {
		remoteRunes := len([]rune(remoteResult))
		ratio := float64(remoteRunes) / float64(len([]rune(sampleText))) * 100
		fmt.Printf("✅ 耗时: %v (%.1f 秒)\n", remoteDuration, remoteDuration.Seconds())
		fmt.Printf("   输入: %d 字 → 输出: %d 字 (%.0f%%)\n", len([]rune(sampleText)), remoteRunes, ratio)
		if remoteResult == sampleText {
			fmt.Println("   输出与原文本完全相同 (未做修改)")
		} else {
			fmt.Printf("   输出预览: %s...\n", truncate(remoteResult, 80))
		}
	}
	fmt.Println()

	// ========== 3. 对比汇总 ==========
	fmt.Println("============================================")
	fmt.Println("  对比汇总")
	fmt.Println("============================================")
	localOk := localErr == nil
	if localOk {
		fmt.Printf("  本地模型:  %v (%.1f 秒)\n", localDuration, localDuration.Seconds())
	} else {
		fmt.Printf("  本地模型:  失败 (%v)\n", localErr)
	}
	remoteOk := remoteErr == nil
	if remoteOk {
		fmt.Printf("  远程API:   %v (%.1f 秒)\n", remoteDuration, remoteDuration.Seconds())
	} else {
		fmt.Printf("  远程API:   失败 (%v)\n", remoteErr)
	}
	if localOk {
		speedup := remoteDuration.Seconds() / localDuration.Seconds()
		if speedup < 1 {
			speedup = localDuration.Seconds() / remoteDuration.Seconds()
			fmt.Printf("  速度比:    远程API 是本地模型的 %.1f 倍\n", speedup)
		} else {
			fmt.Printf("  速度比:    本地模型是远程API的 %.1f 倍\n", speedup)
		}
	}
	fmt.Println()
	fmt.Println("说明: 本地 Ollama 使用 CPU 推理（当前系统无 GPU）")
	fmt.Println("      远程 API 使用 ModelScope 云端服务")
	fmt.Println("============================================")
}

// extractSample 返回测试用的中文小说样本（~1500 字符）
func extractSample() string {
	// 从《恶搞暗黑破坏神》清洗文件中截取的段落
	raw := `圣骑士这回真地火了："谁能把这厮拿下，奖励裂开的黄宝石一枚！"重赏之下必有勇夫。一只巨大野兽走了出来，喝道："不要小看天下英雄！俺来修理他！"圣骑士一开始就有点惧怕穿毛大衣的家伙，没想到是祸躲不过。尸体发火道："这位是本窟2005年摔跤冠军，如果你能将他打败，我就放了你！"圣骑土咽了口唾沫，说："可以动刀不？"尸体发火说："NO！"圣骑土的心拔凉拔凉的……"有酒么？"圣骑士问。他记得有一次自己喝醉了，将路边一只瘸腿狗打咸了半瘫，他想重新找回那种感觉。"给他酒。"尸体发火听说过中国有种叫做"醉八仙"的拳法，杀人于俯仰之间，甚是霸道，便以为今日可以得见神技。要是他也知道"酒壮熊人胆"的谚语，想必会更理解圣骑土的一番苦心。喝了1公升"萝格老窖"，圣骑士果然像换了个人似的，全身散发出逼人的酒气，踩着八仙步，指着那只巨大野兽喝道："你，你，还有你，你们仨一起上吧！"
圣骑土的醉拳连唯一的作用（吓唬人）也没起。享受了38个鞭腿、25个过肩摔和16个麻花大坐之后，圣骑土的头盔和护胸甲耐久度告罄，连保护双肾的腰带也出现了可怕的列痕——圣骑士第一次发现原来自己如此深爱着恰西。肚子里的酒掺和着早上吃的几个变质的饺子吐了巨大野兽一身。尸体发火脸上露出了难得的笑容。生死关头圣骑士冲着对手大喊道："慢着，我有话说……"巨大野兽一愣，曰："有屁快放！""你看过央视版的《笑傲江湖》么？"圣骑士满怀期待地问。巨大野兽说："看过丫。咋啦？"圣骑士大惊失色："那……那那……你看过央视版《射雕》么？""看过丫！少废话！"巨大野兽的耐心显然已经到了极限。"最后一个问题——你吃过四川火锅么？""常吃。去死吧——"巨大野兽的巨拳砸了下来。"完了——"圣骑士闭上了眼睛。突然，巨大野兽的手停在了半空，脸上的表情极度痛苦。`
	// 截取 1500 字符
	runes := []rune(raw)
	if len(runes) > 1500 {
		runes = runes[:1500]
	} else if len(runes) < 1200 {
		// 如果太短，重复到 1200 字以上
		for len(runes) < 1200 {
			runes = append(runes, []rune(raw)...)
		}
		if len(runes) > 1500 {
			runes = runes[:1500]
		}
	}
	return string(runes)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
