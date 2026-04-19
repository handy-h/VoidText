#!/usr/bin/env python3
"""
Evolver适配器脚本 - 调用Evolver CLI进行提示词优化

输入：
  --prompt: 当前提示词
  --context: JSON格式的错误上下文

输出：优化后的提示词（打印到stdout）

工作流程：
1. 将错误上下文写入 memory/errors.jsonl（符合GEP协议格式）
2. 调用 evolver run 扫描memory目录并生成优化建议
3. 解析Evolver输出，提取优化后的提示词
4. 返回纯文本提示词供Go程序使用

依赖：
- Node.js >= 18
- Evolver CLI: npm install -g @evomap/evolver
- Git仓库（项目根目录需已初始化git）
"""

import argparse
import json
import os
import subprocess
import sys
import tempfile
from datetime import datetime
from pathlib import Path

def write_error_to_memory(context_json, base_dir="."):
    """将错误上下文写入memory/errors.jsonl文件"""
    memory_dir = Path(base_dir) / "memory"
    memory_dir.mkdir(exist_ok=True)
    
    error_file = memory_dir / "errors.jsonl"
    
    # 解析输入上下文
    try:
        context = json.loads(context_json)
    except json.JSONDecodeError:
        context = {
            "error_type": "parse_error",
            "raw_context": context_json,
            "timestamp": datetime.now().isoformat()
        }
    
    # 转换为GEP协议格式（简化版）
    gep_event = {
        "type": "error",
        "timestamp": datetime.now().isoformat(),
        "source": "txt-cleaning",
        "data": {
            "prompt_version": context.get("prompt_version", "unknown"),
            "prompt_length": context.get("prompt_length", 0),
            "error_type": context.get("error_type", "unknown"),
            "recommendation": context.get("recommendation", ""),
            "original_context": context
        }
    }
    
    # 写入JSONL文件
    with open(error_file, 'a', encoding='utf-8') as f:
        f.write(json.dumps(gep_event, ensure_ascii=False) + '\n')
    
    return str(error_file)

def run_evolver(base_dir="."):
    """运行Evolver CLI并返回输出"""
    # 切换到项目根目录
    original_cwd = os.getcwd()
    os.chdir(base_dir)
    
    try:
        # 运行evolver run（扫描memory目录并生成优化）
        cmd = ["evolver", "run"]
        result = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            encoding='utf-8',
            timeout=300  # 5分钟超时
        )
        
        if result.returncode != 0:
            # 尝试使用evolver /evolve作为备选
            cmd = ["evolver", "/evolve"]
            result = subprocess.run(
                cmd,
                capture_output=True,
                text=True,
                encoding='utf-8',
                timeout=300
            )
        
        return result.stdout, result.stderr, result.returncode
    finally:
        os.chdir(original_cwd)

def extract_prompt_from_evolver_output(output):
    """从Evolver输出中提取优化后的提示词"""
    # Evolver输出可能是JSON格式的GEP协议，也可能是纯文本提示词
    # 尝试解析JSON
    lines = output.strip().split('\n')
    for line in lines:
        line = line.strip()
        if not line:
            continue
        
        # 尝试解析为JSON
        try:
            data = json.loads(line)
            # 检查是否为GEP协议格式
            if isinstance(data, dict):
                # 查找prompt或content字段
                prompt = data.get('prompt') or data.get('content') or data.get('text')
                if prompt:
                    return str(prompt)
                
                # 检查data字段
                if 'data' in data and isinstance(data['data'], dict):
                    prompt = data['data'].get('prompt') or data['data'].get('content')
                    if prompt:
                        return str(prompt)
        except json.JSONDecodeError:
            # 不是JSON，可能是纯文本提示词
            pass
    
    # 如果没有找到JSON，尝试查找包含"prompt"的行或直接使用最后一行
    for line in lines:
        if 'prompt' in line.lower() and ':' in line:
            # 可能是 "prompt": "..." 格式
            parts = line.split(':', 1)
            if len(parts) == 2:
                prompt = parts[1].strip().strip('"\'')
                if prompt:
                    return prompt
    
    # 如果所有方法都失败，返回原始输出（可能是纯文本提示词）
    return output.strip()

def optimize_prompt_with_evolver(current_prompt, context_json, base_dir="."):
    """使用Evolver优化提示词"""
    # 1. 写入错误日志
    error_file = write_error_to_memory(context_json, base_dir)
    print(f"[Evolver] 错误日志已写入: {error_file}", file=sys.stderr)
    
    # 2. 运行Evolver
    print("[Evolver] 正在运行Evolver优化...", file=sys.stderr)
    stdout, stderr, returncode = run_evolver(base_dir)
    
    if returncode != 0:
        print(f"[Evolver] Evolver执行失败 (code: {returncode}):", file=sys.stderr)
        print(stderr, file=sys.stderr)
        # 回退到基于规则的优化
        return fallback_optimization(current_prompt, context_json)
    
    print(f"[Evolver] Evolver输出长度: {len(stdout)} 字符", file=sys.stderr)
    
    # 3. 提取优化后的提示词
    optimized_prompt = extract_prompt_from_evolver_output(stdout)
    
    if not optimized_prompt or optimized_prompt == current_prompt:
        print("[Evolver] 未能从输出中提取新提示词，使用回退优化", file=sys.stderr)
        return fallback_optimization(current_prompt, context_json)
    
    return optimized_prompt

def fallback_optimization(current_prompt, context_json):
    """回退优化策略（当Evolver失败时使用）"""
    try:
        context = json.loads(context_json)
        error_type = context.get("error_type", "unknown")
        
        base_template = "你是一个专业的中文小说校对编辑。请修正以下段落中的错别字和语法错误，保持原文风格不变。只输出修正后的文本，无需解释。"
        
        if current_prompt.strip() == base_template:
            if error_type == "high_error_rate":
                return f"""{base_template}

重要要求：
1. 只修正明显的错别字和语法错误，不要改变原文意思
2. 保持口语化表达，不要过度正式化
3. 如果原文没有错误，直接返回原文
4. 输出格式：只输出修正后的文本，不要添加任何额外说明

示例：
输入：她高兴及了，跑过去抱住他。
输出：她高兴极了，跑过去抱住他。

当前任务：请修正以下文本："""
            elif error_type == "low_hit_rate":
                return f"""{base_template}

特别注意：
- 修正常见的错别字，如"及了"->"极了"，"在次"->"再次"
- 保持段落完整性，不要拆分或合并段落
- 专有名词（人名、地名）保持原样
- 网络用语和方言根据上下文判断是否修正"""
    
    except Exception as e:
        print(f"[Evolver] 回退优化失败: {e}", file=sys.stderr)
    
    # 默认优化
    return f"""{current_prompt}

请确保：
1. 只修正错误，不改变原文风格和意思
2. 输出格式：仅输出修正后的文本"""

def main():
    parser = argparse.ArgumentParser(description='Evolver适配器 - 提示词优化工具')
    parser.add_argument('--prompt', required=True, help='当前提示词')
    parser.add_argument('--context', required=True, help='JSON错误上下文')
    parser.add_argument('--output', help='输出文件路径（可选）', default=None)
    parser.add_argument('--base-dir', help='项目根目录（默认: .）', default='.')
    
    args = parser.parse_args()
    
    # 优化提示词
    optimized_prompt = optimize_prompt_with_evolver(
        args.prompt, 
        args.context,
        args.base_dir
    )
    
    # 输出优化后的提示词
    print(optimized_prompt)
    
    # 可选：保存到文件
    if args.output:
        with open(args.output, 'w', encoding='utf-8') as f:
            f.write(optimized_prompt)
        
        # 保存优化报告
        report = {
            "timestamp": datetime.now().isoformat(),
            "original_prompt_length": len(args.prompt),
            "optimized_prompt_length": len(optimized_prompt),
            "context": json.loads(args.context) if args.context else {},
            "source": "evolver"
        }
        
        report_path = args.output + '.report.json'
        with open(report_path, 'w', encoding='utf-8') as f:
            json.dump(report, f, ensure_ascii=False, indent=2)
    
    # 记录日志
    print(f"[Evolver] 优化完成，新提示词长度: {len(optimized_prompt)}", file=sys.stderr)

if __name__ == '__main__':
    main()