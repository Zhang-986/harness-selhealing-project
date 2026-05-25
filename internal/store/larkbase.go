package store

import (
	"encoding/json"
	"fmt"
	"harness/internal/model"
	"os/exec"
	"strings"
)

const (
	baseToken    = "ZhSDbHEVuaW99ks8uFrcfdo9nAe"
	promptsTable = "tbl7WZocrcsuRJ5I"
	runsTable    = "tblhDiqp0dMUPP8W"
)

type LarkBaseStore struct{}

func NewLarkBaseStore() *LarkBaseStore {
	return &LarkBaseStore{}
}

type recordList struct {
	OK   bool `json:"ok"`
	Data struct {
		Data      []json.RawMessage `json:"data"`
		Fields    []string          `json:"fields"`
		RecordIDs []string          `json:"record_id_list"`
	} `json:"data"`
}

func (s *LarkBaseStore) ListPrompts() ([]model.Prompt, error) {
	records, err := s.listRecords(promptsTable)
	if err != nil {
		return nil, err
	}
	var result []model.Prompt
	for i, raw := range records.Data.Data {
		m := s.parseRow(records.Data.Fields, raw, records.Data.RecordIDs[i])
		result = append(result, model.Prompt{
			RecordID:       m["_record_id"],
			Name:           m["名称"],
			Category:       m["分类"],
			Template:       m["模板内容"],
			Version:        s.toInt(m["版本"]),
			Status:         m["状态"],
			StrategyLevel:  m["策略层级"],
			CoverageReport: m["覆盖报告"],
			Notes:          m["备注"],
		})
	}
	return result, nil
}

func (s *LarkBaseStore) ListRuns() ([]model.Run, error) {
	records, err := s.listRecords(runsTable)
	if err != nil {
		return nil, err
	}
	var result []model.Run
	for i, raw := range records.Data.Data {
		m := s.parseRow(records.Data.Fields, raw, records.Data.RecordIDs[i])
		result = append(result, model.Run{
			RecordID:     m["_record_id"],
			PromptID:     m["关联提示词"],
			Task:         m["任务描述"],
			Output:       m["输出摘要"],
			Status:       m["状态"],
			EvalSignal:   m["评估信号"],
			EvalMethod:   m["评估方式"],
			Feedback:     m["反馈"],
			Correction:   m["纠偏内容"],
			Iteration:    s.toInt(m["迭代轮次"]),
			NextPromptID: m["下轮提示词"],
			Convergence:  m["收敛状态"],
			CreatedAt:    m["执行时间"],
		})
	}
	return result, nil
}

func (s *LarkBaseStore) GetPrompt(recordID string) (*model.Prompt, error) {
	prompts, err := s.ListPrompts()
	if err != nil {
		return nil, err
	}
	for _, p := range prompts {
		if p.RecordID == recordID {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("prompt %s not found", recordID)
}

func (s *LarkBaseStore) GetRun(recordID string) (*model.Run, error) {
	runs, err := s.ListRuns()
	if err != nil {
		return nil, err
	}
	for _, r := range runs {
		if r.RecordID == recordID {
			return &r, nil
		}
	}
	return nil, fmt.Errorf("run %s not found", recordID)
}

func (s *LarkBaseStore) UpsertPrompt(p *model.Prompt) (string, error) {
	fields := map[string]any{
		"名称":   p.Name,
		"分类":   p.Category,
		"模板内容": p.Template,
		"版本":   p.Version,
		"状态":   p.Status,
		"策略层级": p.StrategyLevel,
		"备注":   p.Notes,
	}
	return s.upsertRecord(promptsTable, p.RecordID, fields)
}

func (s *LarkBaseStore) UpsertRun(r *model.Run) (string, error) {
	fields := map[string]any{
		"任务描述":  r.Task,
		"输出摘要":  r.Output,
		"状态":    r.Status,
		"评估信号":  r.EvalSignal,
		"评估方式":  r.EvalMethod,
		"反馈":    r.Feedback,
		"纠偏内容":  r.Correction,
		"迭代轮次":  r.Iteration,
		"执行时间":  r.CreatedAt,
	}
	if r.PromptID != "" {
		fields["关联提示词"] = []map[string]string{{"id": r.PromptID}}
	}
	if r.NextPromptID != "" {
		fields["下轮提示词"] = r.NextPromptID
	}
	return s.upsertRecord(runsTable, r.RecordID, fields)
}

func (s *LarkBaseStore) UpdateRun(recordID string, fields map[string]any) error {
	_, err := s.upsertRecord(runsTable, recordID, fields)
	return err
}

func (s *LarkBaseStore) UpdatePrompt(recordID string, fields map[string]any) error {
	_, err := s.upsertRecord(promptsTable, recordID, fields)
	return err
}

func (s *LarkBaseStore) FindRunsByPrompt(promptRecordID string) ([]model.Run, error) {
	runs, err := s.ListRuns()
	if err != nil {
		return nil, err
	}
	var result []model.Run
	for _, r := range runs {
		if r.PromptID == promptRecordID {
			result = append(result, r)
		}
	}
	return result, nil
}

func (s *LarkBaseStore) listRecords(tableID string) (*recordList, error) {
	out, err := exec.Command("lark-cli", "base", "+record-list",
		"--base-token", baseToken,
		"--table-id", tableID,
		"--limit", "200",
	).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s: %s", err, string(out))
	}
	var result recordList
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *LarkBaseStore) upsertRecord(tableID string, recordID string, fields map[string]any) (string, error) {
	data, _ := json.Marshal(fields)
	args := []string{"base", "+record-upsert",
		"--base-token", baseToken,
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
	var result struct {
		Record struct {
			RecordIDs []string `json:"record_id_list"`
		} `json:"record"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return "", err
	}
	if len(result.Record.RecordIDs) > 0 {
		return result.Record.RecordIDs[0], nil
	}
	return "", nil
}

func (s *LarkBaseStore) parseRow(fields []string, raw json.RawMessage, recordID string) map[string]string {
	var row []any
	if err := json.Unmarshal(raw, &row); err != nil {
		return map[string]string{"_record_id": recordID}
	}
	m := map[string]string{"_record_id": recordID}
	for i, f := range fields {
		if i < len(row) {
			m[f] = s.toString(row[i])
		}
	}
	return m
}

func (s *LarkBaseStore) toString(v any) string {
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

func (s *LarkBaseStore) toInt(v string) int {
	var n int
	fmt.Sscanf(v, "%d", &n)
	return n
}
