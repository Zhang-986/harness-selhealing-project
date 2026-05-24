package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"
)

const (
	BaseToken    = "ZhSDbHEVuaW99ks8uFrcfdo9nAe"
	PromptsTable = "tbl7WZocrcsuRJ5I"
	RunsTable    = "tblhDiqp0dMUPP8W"
)

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
	default:
		printUsage()
	}
}

func printUsage() {
	fmt.Println(`harness - 提示词与执行管理工具 (多维表格版)

Usage:
  harness prompts              列出所有提示词
  harness runs                 列出所有执行记录
  harness add-prompt           添加新提示词
  harness add-run              添加执行记录
  harness iterate <record_id>  查看某次执行的迭代链`)
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

func upsertRecord(tableID string, fields map[string]any) (string, error) {
	data, _ := json.Marshal(fields)
	out, err := exec.Command("lark-cli", "base", "+record-upsert",
		"--base-token", BaseToken,
		"--table-id", tableID,
		"--json", string(data),
	).CombinedOutput()
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
		return fmt.Sprintf("%v", parts)
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

func listPrompts() {
	result, err := listRecords(PromptsTable)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取失败: %v\n", err)
		return
	}
	if len(result.Data.Data) == 0 {
		fmt.Println("暂无提示词")
		return
	}
	for i, row := range result.Data.Data {
		m := mapFields(result.Data.Fields, row, result.Data.RecordIDs[i])
		fmt.Printf("%s | %s | %s | v%s | %s\n",
			m["_record_id"][:8], m["名称"], m["分类"], m["版本"], m["状态"])
	}
}

func listRuns() {
	result, err := listRecords(RunsTable)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取失败: %v\n", err)
		return
	}
	if len(result.Data.Data) == 0 {
		fmt.Println("暂无执行记录")
		return
	}
	for i, row := range result.Data.Data {
		m := mapFields(result.Data.Fields, row, result.Data.RecordIDs[i])
		fmt.Printf("%s | iter=%s | %s | %s\n",
			m["_record_id"][:8], m["迭代轮次"], m["状态"], m["任务描述"])
	}
}

func addPrompt() {
	var name, category, template, notes string
	fmt.Print("名称: ")
	fmt.Scanln(&name)
	fmt.Print("分类 (test_case/e2e/other): ")
	fmt.Scanln(&category)
	fmt.Print("模板内容: ")
	fmt.Scanln(&template)
	fmt.Print("备注: ")
	fmt.Scanln(&notes)

	fields := map[string]any{
		"名称":   name,
		"分类":   category,
		"模板内容": template,
		"版本":   1,
		"状态":   "draft",
		"备注":   notes,
	}
	id, err := upsertRecord(PromptsTable, fields)
	if err != nil {
		fmt.Fprintf(os.Stderr, "添加失败: %v\n", err)
		return
	}
	fmt.Printf("已添加提示词 record_id=%s\n", id)
}

func addRun() {
	var promptRecordID, task, summary, status, feedback, correction string
	fmt.Print("关联提示词 record_id: ")
	fmt.Scanln(&promptRecordID)
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

	fields := map[string]any{
		"关联提示词": []map[string]string{{"id": promptRecordID}},
		"任务描述":  task,
		"输出摘要":  summary,
		"状态":    status,
		"反馈":    feedback,
		"纠偏内容":  correction,
		"迭代轮次":  1,
		"执行时间":  time.Now().Format("2006-01-02 15:04"),
	}
	id, err := upsertRecord(RunsTable, fields)
	if err != nil {
		fmt.Fprintf(os.Stderr, "添加失败: %v\n", err)
		return
	}
	fmt.Printf("已添加执行记录 record_id=%s\n", id)
}

func showIterationChain() {
	if len(os.Args) < 3 {
		fmt.Println("用法: harness iterate <record_id>")
		return
	}
	runID := os.Args[2]

	result, err := listRecords(RunsTable)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取失败: %v\n", err)
		return
	}
	if len(result.Data.Data) == 0 {
		fmt.Println("暂无执行记录")
		return
	}

	var records []map[string]string
	for i, row := range result.Data.Data {
		records = append(records, mapFields(result.Data.Fields, row, result.Data.RecordIDs[i]))
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
		fmt.Printf("[%s] iter=%s status=%s\n", r["_record_id"][:8], r["迭代轮次"], r["状态"])
		fmt.Printf("  任务: %s\n", r["任务描述"])
		if r["反馈"] != "" {
			fmt.Printf("  反馈: %s\n", r["反馈"])
		}
		if r["纠偏内容"] != "" {
			fmt.Printf("  纠偏: %s\n", r["纠偏内容"])
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
			fmt.Printf("  → 下轮提示词 %s 尚无执行记录\n", nextPromptID[:8])
			break
		}
	}
}
