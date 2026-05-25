package main

import (
	"encoding/json"
	"fmt"
	"harness/internal/evaluator"
	"harness/internal/model"
	"harness/internal/service"
	"harness/internal/store"
	"os"
	"time"
)

func main() {
	svc := service.NewHarnessService(
		store.NewLarkBaseStore(),
		evaluator.NewRegistry(),
	)

	if len(os.Args) < 2 {
		printUsage()
		return
	}

	switch os.Args[1] {
	case "prompt":
		handlePrompt(svc, os.Args[2:])
	case "run":
		handleRun(svc, os.Args[2:])
	case "evaluate":
		handleEvaluate(svc, os.Args[2:])
	case "convergence":
		handleConvergence(svc)
	case "strategy":
		handleStrategy(svc, os.Args[2:])
	case "iterate":
		handleIterate(svc, os.Args[2:])
	default:
		printUsage()
	}
}

func printUsage() {
	fmt.Println(`harness - Agent 自愈循环管理框架

Usage:
  harness prompt list                              列出所有提示词
  harness prompt get <id>                          查看提示词详情
  harness prompt create                            交互式创建提示词

  harness run list                                 列出所有执行记录
  harness run get <id>                             查看执行记录详情
  harness run create                               交互式创建执行记录

  harness evaluate <run_id>                        自动评估执行结果
  harness convergence                              检查所有迭代链收敛状态
  harness strategy <prompt_id>                     建议下一步策略升级
  harness iterate <run_id>                         查看迭代链

Flags:
  --json    结构化 JSON 输出`)
}

func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

func handlePrompt(svc *service.HarnessService, args []string) {
	if len(args) < 1 {
		fmt.Println("用法: harness prompt <list|get|create>")
		return
	}
	jsonOutput := hasFlag(args, "--json")

	switch args[0] {
	case "list":
		prompts, err := svc.ListPrompts()
		if err != nil {
			fmt.Fprintf(os.Stderr, "错误: %v\n", err)
			return
		}
		if jsonOutput {
			data, _ := json.MarshalIndent(prompts, "", "  ")
			fmt.Println(string(data))
			return
		}
		for _, p := range prompts {
			id := shortID(p.RecordID)
			slug := p.StrategyLevel
			if slug == "" {
				slug = "-"
			}
			fmt.Printf("%s | %s | %s | v%d | %s | %s\n", id, p.Name, p.Category, p.Version, p.Status, slug)
		}
	case "get":
		if len(args) < 2 {
			fmt.Println("用法: harness prompt get <id>")
			return
		}
		p, err := svc.GetPrompt(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "错误: %v\n", err)
			return
		}
		if jsonOutput {
			data, _ := json.MarshalIndent(p, "", "  ")
			fmt.Println(string(data))
			return
		}
		fmt.Printf("ID:     %s\n", p.RecordID)
		fmt.Printf("名称:   %s\n", p.Name)
		fmt.Printf("分类:   %s\n", p.Category)
		fmt.Printf("版本:   %d\n", p.Version)
		fmt.Printf("状态:   %s\n", p.Status)
		fmt.Printf("策略:   %s\n", p.StrategyLevel)
		fmt.Printf("覆盖:   %s\n", p.CoverageReport)
		fmt.Printf("模板:   %s\n", truncate(p.Template, 200))
	case "create":
		var name, category, template, strategy, notes string
		fmt.Print("名称: ")
		fmt.Scanln(&name)
		fmt.Print("分类 (test_case/e2e/other): ")
		fmt.Scanln(&category)
		fmt.Print("模板内容: ")
		fmt.Scanln(&template)
		fmt.Print("策略层级 (basic/constrained/few_shot/strict): ")
		fmt.Scanln(&strategy)
		fmt.Print("备注: ")
		fmt.Scanln(&notes)
		id, err := svc.CreatePrompt(&model.Prompt{
			Name:          name,
			Category:      category,
			Template:      template,
			StrategyLevel: strategy,
			Notes:         notes,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "错误: %v\n", err)
			return
		}
		fmt.Printf("已创建 prompt: %s\n", id)
	}
}

func handleRun(svc *service.HarnessService, args []string) {
	if len(args) < 1 {
		fmt.Println("用法: harness run <list|get|create>")
		return
	}
	jsonOutput := hasFlag(args, "--json")

	switch args[0] {
	case "list":
		runs, err := svc.ListRuns()
		if err != nil {
			fmt.Fprintf(os.Stderr, "错误: %v\n", err)
			return
		}
		if jsonOutput {
			data, _ := json.MarshalIndent(runs, "", "  ")
			fmt.Println(string(data))
			return
		}
		for _, r := range runs {
			id := shortID(r.RecordID)
			conv := r.Convergence
			if conv == "" {
				conv = "-"
			}
			fmt.Printf("%s | iter=%d | %s | %s | conv=%s\n", id, r.Iteration, r.Status, truncate(r.Task, 30), conv)
		}
	case "get":
		if len(args) < 2 {
			fmt.Println("用法: harness run get <id>")
			return
		}
		r, err := svc.GetRun(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "错误: %v\n", err)
			return
		}
		if jsonOutput {
			data, _ := json.MarshalIndent(r, "", "  ")
			fmt.Println(string(data))
			return
		}
		fmt.Printf("ID:     %s\n", r.RecordID)
		fmt.Printf("任务:   %s\n", r.Task)
		fmt.Printf("状态:   %s\n", r.Status)
		fmt.Printf("评估:   %s (%s)\n", r.EvalSignal, r.EvalMethod)
		fmt.Printf("反馈:   %s\n", r.Feedback)
		fmt.Printf("纠偏:   %s\n", r.Correction)
		fmt.Printf("迭代:   %d\n", r.Iteration)
		fmt.Printf("收敛:   %s\n", r.Convergence)
	case "create":
		var promptID, task, summary, status, feedback, correction string
		fmt.Print("关联提示词 record_id: ")
		fmt.Scanln(&promptID)
		fmt.Print("任务描述: ")
		fmt.Scanln(&task)
		fmt.Print("输出摘要: ")
		fmt.Scanln(&summary)
		fmt.Print("状态 (ok/partial/fail/pending): ")
		fmt.Scanln(&status)
		fmt.Print("反馈: ")
		fmt.Scanln(&feedback)
		fmt.Print("纠偏内容: ")
		fmt.Scanln(&correction)
		id, err := svc.CreateRun(&model.Run{
			PromptID:   promptID,
			Task:       task,
			Output:     summary,
			Status:     status,
			Feedback:   feedback,
			Correction: correction,
			CreatedAt:  time.Now().Format("2006-01-02 15:04"),
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "错误: %v\n", err)
			return
		}
		fmt.Printf("已创建 run: %s\n", id)
	}
}

func handleEvaluate(svc *service.HarnessService, args []string) {
	if len(args) < 1 {
		fmt.Println("用法: harness evaluate <run_id>")
		return
	}
	result, err := svc.EvaluateRun(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		return
	}
	fmt.Printf("评估信号: %s\n", result.Signal)
	fmt.Printf("评估方式: %s\n", result.Method)
	fmt.Printf("是否通过: %v\n", result.Pass)
	if result.Details != "" {
		fmt.Printf("缺失项:   %s\n", result.Details)
	}
}

func handleConvergence(svc *service.HarnessService) {
	result, err := svc.CheckConvergence()
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		return
	}
	for pid, status := range result {
		id := shortID(pid)
		fmt.Printf("prompt %s: %s", id, status)
		if status == model.Stuck {
			fmt.Print(" ⚠️ 止损触发！建议人工介入")
		}
		fmt.Println()
	}
}

func handleStrategy(svc *service.HarnessService, args []string) {
	if len(args) < 1 {
		fmt.Println("用法: harness strategy <prompt_id>")
		return
	}
	prompt, next, hint, err := svc.SuggestNextStrategy(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		return
	}
	fmt.Printf("当前策略: %s → 建议升级到: %s\n", prompt.StrategyLevel, next)
	fmt.Printf("升级动作: %s\n", hint)
}

func handleIterate(svc *service.HarnessService, args []string) {
	if len(args) < 1 {
		fmt.Println("用法: harness iterate <run_id>")
		return
	}
	chain, err := svc.IterateChain(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		return
	}
	fmt.Printf("=== 迭代链 (从 %s 开始) ===\n", shortID(args[0]))
	for _, r := range chain {
		fmt.Printf("[%s] iter=%d status=%s\n", shortID(r.RecordID), r.Iteration, r.Status)
		fmt.Printf("  任务: %s\n", r.Task)
		if r.EvalSignal != "" {
			fmt.Printf("  评估: %s (%s)\n", r.EvalSignal, r.EvalMethod)
		}
		if r.Feedback != "" {
			fmt.Printf("  反馈: %s\n", r.Feedback)
		}
		if r.Correction != "" {
			fmt.Printf("  纠偏: %s\n", r.Correction)
		}
		if r.Convergence != "" {
			fmt.Printf("  收敛: %s\n", r.Convergence)
		}
	}
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
