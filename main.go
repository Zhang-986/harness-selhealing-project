package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	BaseToken          = "ZhSDbHEVuaW99ks8uFrcfdo9nAe"
	PromptsTable       = "tbl7WZocrcsuRJ5I"
	RunsTable          = "tblhDiqp0dMUPP8W"
	MaxConsecutiveFail = 3
)

var strategyLevels = []string{"basic", "constrained", "few_shot", "strict"}

type baseRecordList struct {
	OK   bool `json:"ok"`
	Data struct {
		Data      []json.RawMessage `json:"data"`
		Fields    []string          `json:"fields"`
		RecordIDs []string          `json:"record_id_list"`
		HasMore   bool              `json:"has_more"`
	} `json:"data"`
}

type baseRecordUpsert struct {
	OK      bool `json:"ok"`
	Created bool `json:"created"`
	Record  struct {
		RecordIDs []string `json:"record_id_list"`
	} `json:"record"`
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	switch os.Args[1] {
	case "prompts":
		listPrompts()
	case "runs":
		listRuns()
	case "add-prompt":
		addPrompt()
	case "add-run":
		addRun()
	case "iterate":
		showIterationChain()
	case "evaluate":
		evaluateRun()
	case "check-convergence":
		checkConvergence()
	case "next-strategy":
		suggestNextStrategy()
	default:
		printUsage()
	}
}

func printUsage() {
	fmt.Println(`harness - Agent 自愈循环管理工具

Usage:
  harness prompts                    列出所有提示词
  harness runs                       列出所有执行记录
  harness add-prompt                 添加新提示词
  harness add-run                    添加执行记录
  harness iterate <record_id>        查看迭代链
  harness evaluate <record_id>       自动评估执行结果
  harness check-convergence          检查所有迭代链的收敛状态
  harness next-strategy <record_id>  建议下一步策略升级`)
}

func listRecords(tableID string) (*baseRecordList, error) {
	out, err := exec.Command("lark-cli", "base", "+record-list",
		"--base-token", BaseToken,
		"--table-id", tableID,
		"--limit", "200",
	).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s: %s", err, string(out))
	}
	var result baseRecordList
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func upsertRecord(tableID string, recordID string, fields map[string]any) (string, error) {
	data, _ := json.Marshal(fields)
	args := []string{"base", "+record-upsert",
		"--base-token", BaseToken,
		"--table-id", tableID,
		"--json", string(data),
	}
	if recordID != "" {
		args = append(args, "--record-id", recordID)
	}
	out, err := exec.Command("lark-cli", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %s", err, string(out))
	}
	var result baseRecordUpsert
	if err := json.Unmarshal(out, &result); err != nil {
		return "", err
	}
	if len(result.Record.RecordIDs) > 0 {
		return result.Record.RecordIDs[0], nil
	}
	return "", nil
}

func toString(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case float64:
		if val == float64(int(val)) {
			return fmt.Sprintf("%d", int(val))
		}
		return fmt.Sprintf("%v", val)
	case []any:
		if len(val) == 1 {
			if inner, ok := val[0].(string); ok {
				return inner
			}
			if obj, ok := val[0].(map[string]any); ok {
				if id, ok := obj["id"].(string); ok {
					return id
				}
			}
		}
		parts := make([]string, len(val))
		for i, item := range val {
			parts[i] = fmt.Sprintf("%v", item)
		}
		return strings.Join(parts, ",")
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", val)
	}
}

func mapFields(fields []string, raw json.RawMessage, recordID string) map[string]string {
	var row []any
	if err := json.Unmarshal(raw, &row); err != nil {
		return map[string]string{"_record_id": recordID}
	}
	m := map[string]string{"_record_id": recordID}
	for i, f := range fields {
		if i < len(row) {
			m[f] = toString(row[i])
		}
	}
	return m
}

func loadAllRecords(tableID string) ([]map[string]string, error) {
	result, err := listRecords(tableID)
	if err != nil {
		return nil, err
	}
	var records []map[string]string
	for i, row := range result.Data.Data {
		records = append(records, mapFields(result.Data.Fields, row, result.Data.RecordIDs[i]))
	}
	return records, nil
}

func listPrompts() {
	records, err := loadAllRecords(PromptsTable)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取失败: %v\n", err)
		return
	}
	if len(records) == 0 {
		fmt.Println("暂无提示词")
		return
	}
	for _, m := range records {
		id := m["_record_id"]
		if len(id) > 8 {
			id = id[:8]
		}
		strategy := m["策略层级"]
		if strategy == "" {
			strategy = "-"
		}
		fmt.Printf("%s | %s | %s | v%s | %s | %s\n",
			id, m["名称"], m["分类"], m["版本"], m["状态"], strategy)
	}
}

func listRuns() {
	records, err := loadAllRecords(RunsTable)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取失败: %v\n", err)
		return
	}
	if len(records) == 0 {
		fmt.Println("暂无执行记录")
		return
	}
	for _, m := range records {
		id := m["_record_id"]
		if len(id) > 8 {
			id = id[:8]
		}
		convergence := m["收敛状态"]
		if convergence == "" {
			convergence = "-"
		}
		fmt.Printf("%s | iter=%s | %s | %s | conv=%s\n",
			id, m["迭代轮次"], m["状态"], m["任务描述"], convergence)
	}
}

func addPrompt() {
	var name, category, template, notes, strategy string
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

	if strategy == "" {
		strategy = "basic"
	}
	fields := map[string]any{
		"名称":   name,
		"分类":   category,
		"模板内容": template,
		"版本":   1,
		"状态":   "draft",
		"策略层级": strategy,
		"备注":   notes,
	}
	id, err := upsertRecord(PromptsTable, "", fields)
	if err != nil {
		fmt.Fprintf(os.Stderr, "添加失败: %v\n", err)
		return
	}
	fmt.Printf("已添加提示词 record_id=%s\n", id)
}

func addRun() {
	var promptRecordID, task, summary, status, feedback, correction, evalSignal, evalMethod string
	fmt.Print("关联提示词 record_id: ")
	fmt.Scanln(&promptRecordID)
	fmt.Print("任务描述: ")
	fmt.Scanln(&task)
	fmt.Print("输出摘要: ")
	fmt.Scanln(&summary)
	fmt.Print("状态 (ok/partial/fail/pending): ")
	fmt.Scanln(&status)
	fmt.Print("评估信号 (如 scenario_coverage=4/6): ")
	fmt.Scanln(&evalSignal)
	fmt.Print("评估方式 (auto_rule/auto_run/manual): ")
	fmt.Scanln(&evalMethod)
	fmt.Print("反馈: ")
	fmt.Scanln(&feedback)
	fmt.Print("纠偏内容: ")
	fmt.Scanln(&correction)

	fields := map[string]any{
		"关联提示词": []map[string]string{{"id": promptRecordID}},
		"任务描述":  task,
		"输出摘要":  summary,
		"状态":    status,
		"评估信号":  evalSignal,
		"评估方式":  evalMethod,
		"反馈":    feedback,
		"纠偏内容":  correction,
		"迭代轮次":  1,
		"执行时间":  time.Now().Format("2006-01-02 15:04"),
	}
	id, err := upsertRecord(RunsTable, "", fields)
	if err != nil {
		fmt.Fprintf(os.Stderr, "添加失败: %v\n", err)
		return
	}
	fmt.Printf("已添加执行记录 record_id=%s\n", id)
}

func evaluateRun() {
	if len(os.Args) < 3 {
		fmt.Println("用法: harness evaluate <record_id>")
		return
	}
	runID := os.Args[2]

	records, err := loadAllRecords(RunsTable)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取失败: %v\n", err)
		return
	}

	var target *map[string]string
	for _, r := range records {
		if r["_record_id"] == runID {
			target = &r
			break
		}
	}
	if target == nil {
		fmt.Printf("未找到执行记录 %s\n", runID)
		return
	}
	r := *target

	category := ""
	promptRecords, _ := loadAllRecords(PromptsTable)
	for _, p := range promptRecords {
		if p["_record_id"] == r["关联提示词"] {
			category = p["分类"]
			break
		}
	}

	output := r["输出摘要"]
	var signals []string
	var evalMethod string

	switch category {
	case "test_case":
		requiredKeywords := []string{"批量", "权限", "异常", "自定义", "模板", "联动"}
		var covered []string
		var missing []string
		for _, kw := range requiredKeywords {
			if strings.Contains(output, kw) {
				covered = append(covered, kw)
			} else {
				missing = append(missing, kw)
			}
		}
		signals = append(signals, fmt.Sprintf("scenario_coverage=%d/%d", len(covered), len(requiredKeywords)))
		if len(missing) > 0 {
			signals = append(signals, fmt.Sprintf("missing=%s", strings.Join(missing, ",")))
		}
		evalMethod = "auto_rule"

	case "e2e":
		badPatterns := []string{"data-testid", "sleep(", "setTimeout("}
		goodPatterns := []string{"aria-label", "role=", "waitForSelector", "waitForURL", "networkIdle"}
		var badFound, goodFound []string
		for _, p := range badPatterns {
			if strings.Contains(output, p) {
				badFound = append(badFound, p)
			}
		}
		for _, p := range goodPatterns {
			if strings.Contains(output, p) {
				goodFound = append(goodFound, p)
			}
		}
		signals = append(signals, fmt.Sprintf("good_patterns=%d", len(goodFound)))
		if len(badFound) > 0 {
			signals = append(signals, fmt.Sprintf("bad_patterns=%s", strings.Join(badFound, ",")))
		}
		evalMethod = "auto_rule"

	default:
		signals = append(signals, "manual_review_required")
		evalMethod = "manual"
	}

	signalStr := strings.Join(signals, "; ")
	fmt.Printf("评估结果: %s\n", signalStr)
	fmt.Printf("评估方式: %s\n", evalMethod)

	_, err = upsertRecord(RunsTable, runID, map[string]any{
		"评估信号": signalStr,
		"评估方式": evalMethod,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "更新失败: %v\n", err)
		return
	}
	fmt.Printf("已更新执行记录 %s 的评估信息\n", runID[:8])
}

func checkConvergence() {
	records, err := loadAllRecords(RunsTable)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取失败: %v\n", err)
		return
	}

	promptRuns := map[string][]map[string]string{}
	for _, r := range records {
		pid := r["关联提示词"]
		promptRuns[pid] = append(promptRuns[pid], r)
	}

	for pid, runs := range promptRuns {
		consecutiveFail := 0
		convergence := "converging"

		for _, r := range runs {
			status := r["状态"]
			if status == "ok" {
				consecutiveFail = 0
				convergence = "converged"
			} else if status == "partial" || status == "fail" {
				consecutiveFail++
				if consecutiveFail >= MaxConsecutiveFail {
					convergence = "stuck"
				} else {
					convergence = "converging"
				}
			}
		}

		for _, r := range runs {
			if r["收敛状态"] != convergence {
				_, _ = upsertRecord(RunsTable, r["_record_id"], map[string]any{
					"收敛状态": convergence,
				})
			}
		}

		id := pid
		if len(id) > 8 {
			id = id[:8]
		}
		fmt.Printf("prompt %s: %s (连续失败=%d, runs=%d)\n", id, convergence, consecutiveFail, len(runs))

		if convergence == "stuck" {
			fmt.Printf("  ⚠️  止损触发！连续 %d 次未通过，建议人工介入\n", consecutiveFail)
		}
	}
}

func suggestNextStrategy() {
	if len(os.Args) < 3 {
		fmt.Println("用法: harness next-strategy <prompt_record_id>")
		return
	}
	promptID := os.Args[2]

	promptRecords, err := loadAllRecords(PromptsTable)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取失败: %v\n", err)
		return
	}

	var target *map[string]string
	for _, p := range promptRecords {
		if p["_record_id"] == promptID {
			target = &p
			break
		}
	}
	if target == nil {
		fmt.Printf("未找到提示词 %s\n", promptID)
		return
	}
	p := *target

	currentLevel := p["策略层级"]
	if currentLevel == "" {
		currentLevel = "basic"
	}

	currentIdx := -1
	for i, s := range strategyLevels {
		if s == currentLevel {
			currentIdx = i
			break
		}
	}

	if currentIdx >= len(strategyLevels)-1 {
		fmt.Printf("当前策略层级已是最高 (%s)，无法再升级\n", currentLevel)
		fmt.Println("建议：人工审查 prompt，或拆分任务")
		return
	}

	nextLevel := strategyLevels[currentIdx+1]
	fmt.Printf("当前策略: %s → 建议升级到: %s\n", currentLevel, nextLevel)

	switch nextLevel {
	case "constrained":
		fmt.Println("升级动作：在 prompt 中增加场景枚举约束、格式要求、边界条件")
	case "few_shot":
		fmt.Println("升级动作：在 prompt 中增加 2-3 个高质量示例（few-shot）")
	case "strict":
		fmt.Println("升级动作：增加反例、输出格式校验规则、禁止项清单")
	}

	runRecords, _ := loadAllRecords(RunsTable)
	var relatedRuns []map[string]string
	for _, r := range runRecords {
		if r["关联提示词"] == promptID {
			relatedRuns = append(relatedRuns, r)
		}
	}
	if len(relatedRuns) > 0 {
		lastRun := relatedRuns[len(relatedRuns)-1]
		fmt.Printf("\n最近一次执行: status=%s\n", lastRun["状态"])
		if lastRun["评估信号"] != "" {
			fmt.Printf("评估信号: %s\n", lastRun["评估信号"])
		}
		if lastRun["反馈"] != "" {
			fmt.Printf("反馈: %s\n", lastRun["反馈"])
		}
	}
}

func showIterationChain() {
	if len(os.Args) < 3 {
		fmt.Println("用法: harness iterate <record_id>")
		return
	}
	runID := os.Args[2]

	records, err := loadAllRecords(RunsTable)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取失败: %v\n", err)
		return
	}
	if len(records) == 0 {
		fmt.Println("暂无执行记录")
		return
	}

	var current *map[string]string
	for _, r := range records {
		if r["_record_id"] == runID {
			current = &r
			break
		}
	}
	if current == nil {
		fmt.Printf("未找到执行记录 %s\n", runID)
		return
	}

	fmt.Printf("=== 迭代链 (从 %s 开始) ===\n", runID[:8])
	r := *current
	for {
		id := r["_record_id"]
		if len(id) > 8 {
			id = id[:8]
		}
		fmt.Printf("[%s] iter=%s status=%s\n", id, r["迭代轮次"], r["状态"])
		fmt.Printf("  任务: %s\n", r["任务描述"])
		if r["评估信号"] != "" {
			fmt.Printf("  评估: %s (%s)\n", r["评估信号"], r["评估方式"])
		}
		if r["反馈"] != "" {
			fmt.Printf("  反馈: %s\n", r["反馈"])
		}
		if r["纠偏内容"] != "" {
			fmt.Printf("  纠偏: %s\n", r["纠偏内容"])
		}
		if r["收敛状态"] != "" {
			fmt.Printf("  收敛: %s\n", r["收敛状态"])
		}

		if r["下轮提示词"] == "" {
			break
		}
		nextPromptID := r["下轮提示词"]
		found := false
		for _, candidate := range records {
			if candidate["关联提示词"] == nextPromptID && candidate["_record_id"] != r["_record_id"] {
				r = candidate
				found = true
				break
			}
		}
		if !found {
			nextID := nextPromptID
			if len(nextID) > 8 {
				nextID = nextID[:8]
			}
			fmt.Printf("  → 下轮提示词 %s 尚无执行记录\n", nextID)
			break
		}
	}
}
