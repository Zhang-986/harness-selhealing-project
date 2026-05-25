package service

import (
	"fmt"
	"harness/internal/evaluator"
	"harness/internal/model"
	"harness/internal/store"
)

const maxConsecutiveFail = 3

type HarnessService struct {
	store     store.Store
	evaluator *evaluator.Registry
}

func NewHarnessService(s store.Store, e *evaluator.Registry) *HarnessService {
	return &HarnessService{store: s, evaluator: e}
}

func (svc *HarnessService) ListPrompts() ([]model.Prompt, error) {
	return svc.store.ListPrompts()
}

func (svc *HarnessService) ListRuns() ([]model.Run, error) {
	return svc.store.ListRuns()
}

func (svc *HarnessService) GetPrompt(recordID string) (*model.Prompt, error) {
	return svc.store.GetPrompt(recordID)
}

func (svc *HarnessService) GetRun(recordID string) (*model.Run, error) {
	return svc.store.GetRun(recordID)
}

func (svc *HarnessService) CreatePrompt(p *model.Prompt) (string, error) {
	if p.StrategyLevel == "" {
		p.StrategyLevel = string(model.Basic)
	}
	if p.Version == 0 {
		p.Version = 1
	}
	if p.Status == "" {
		p.Status = "draft"
	}
	return svc.store.UpsertPrompt(p)
}

func (svc *HarnessService) CreateRun(r *model.Run) (string, error) {
	if r.Iteration == 0 {
		r.Iteration = 1
	}
	return svc.store.UpsertRun(r)
}

func (svc *HarnessService) EvaluateRun(runID string) (*model.EvalResult, error) {
	run, err := svc.store.GetRun(runID)
	if err != nil {
		return nil, err
	}

	prompt, err := svc.store.GetPrompt(run.PromptID)
	category := ""
	if err == nil {
		category = prompt.Category
	}

	result := svc.evaluator.Evaluate(category, run)

	err = svc.store.UpdateRun(runID, map[string]any{
		"评估信号": result.Signal,
		"评估方式": result.Method,
	})
	if err != nil {
		return nil, err
	}

	return &result, nil
}

func (svc *HarnessService) CheckConvergence() (map[string]model.ConvergenceStatus, error) {
	runs, err := svc.store.ListRuns()
	if err != nil {
		return nil, err
	}

	promptRuns := map[string][]model.Run{}
	for _, r := range runs {
		promptRuns[r.PromptID] = append(promptRuns[r.PromptID], r)
	}

	result := map[string]model.ConvergenceStatus{}
	for pid, pruns := range promptRuns {
		consecutiveFail := 0
		status := model.Converging

		for _, r := range pruns {
			if r.Status == "ok" {
				consecutiveFail = 0
				status = model.Converged
			} else if r.Status == "partial" || r.Status == "fail" {
				consecutiveFail++
				if consecutiveFail >= maxConsecutiveFail {
					status = model.Stuck
				} else {
					status = model.Converging
				}
			}
		}

		result[pid] = status

		for _, r := range pruns {
			if r.Convergence != string(status) {
				_ = svc.store.UpdateRun(r.RecordID, map[string]any{
					"收敛状态": string(status),
				})
			}
		}
	}

	return result, nil
}

func (svc *HarnessService) SuggestNextStrategy(promptID string) (*model.Prompt, model.StrategyLevel, string, error) {
	prompt, err := svc.store.GetPrompt(promptID)
	if err != nil {
		return nil, "", "", err
	}

	current := model.StrategyLevel(prompt.StrategyLevel)
	if current == "" {
		current = model.Basic
	}

	if current.IsMax() {
		return prompt, current, "已是最高层级，建议人工审查或拆分任务", nil
	}

	next := current.Next()
	hint := next.UpgradeHint()
	return prompt, next, hint, nil
}

func (svc *HarnessService) IterateChain(runID string) ([]model.Run, error) {
	runs, err := svc.store.ListRuns()
	if err != nil {
		return nil, err
	}

	runMap := map[string]model.Run{}
	for _, r := range runs {
		runMap[r.RecordID] = r
	}

	var chain []model.Run
	current, ok := runMap[runID]
	if !ok {
		return nil, fmt.Errorf("run %s not found", runID)
	}

	chain = append(chain, current)
	for current.NextPromptID != "" {
		found := false
		for _, r := range runs {
			if r.PromptID == current.NextPromptID && r.RecordID != current.RecordID {
				chain = append(chain, r)
				current = r
				found = true
				break
			}
		}
		if !found {
			break
		}
	}

	return chain, nil
}
