package evaluator

import (
	"fmt"
	"harness/internal/model"
	"strings"
)

type Evaluator interface {
	Category() string
	Evaluate(run *model.Run) model.EvalResult
}

type Registry struct {
	evaluators map[string]Evaluator
}

func NewRegistry() *Registry {
	r := &Registry{evaluators: make(map[string]Evaluator)}
	r.Register(&TestCaseEvaluator{})
	r.Register(&E2EEvaluator{})
	return r
}

func (r *Registry) Register(e Evaluator) {
	r.evaluators[e.Category()] = e
}

func (r *Registry) Evaluate(category string, run *model.Run) model.EvalResult {
	if e, ok := r.evaluators[category]; ok {
		return e.Evaluate(run)
	}
	return model.EvalResult{
		Signal: "manual_review_required",
		Method: "manual",
		Pass:   false,
	}
}

type TestCaseEvaluator struct{}

func (e *TestCaseEvaluator) Category() string { return "test_case" }

func (e *TestCaseEvaluator) Evaluate(run *model.Run) model.EvalResult {
	requiredKeywords := []string{"批量", "权限", "异常", "自定义", "模板", "联动"}
	var covered, missing []string
	for _, kw := range requiredKeywords {
		if strings.Contains(run.Output, kw) {
			covered = append(covered, kw)
		} else {
			missing = append(missing, kw)
		}
	}
	signal := fmt.Sprintf("scenario_coverage=%d/%d", len(covered), len(requiredKeywords))
	if len(missing) > 0 {
		signal += fmt.Sprintf("; missing=%s", strings.Join(missing, ","))
	}
	return model.EvalResult{
		Signal:  signal,
		Method:  "auto_rule",
		Pass:    len(missing) == 0,
		Details: strings.Join(missing, ","),
	}
}

type E2EEvaluator struct{}

func (e *E2EEvaluator) Category() string { return "e2e" }

func (e *E2EEvaluator) Evaluate(run *model.Run) model.EvalResult {
	badPatterns := []string{"data-testid", "sleep(", "setTimeout("}
	goodPatterns := []string{"aria-label", "role=", "waitForSelector", "waitForURL", "networkIdle", "Page Object"}
	var badFound, goodFound []string
	for _, p := range badPatterns {
		if strings.Contains(run.Output, p) {
			badFound = append(badFound, p)
		}
	}
	for _, p := range goodPatterns {
		if strings.Contains(run.Output, p) {
			goodFound = append(goodFound, p)
		}
	}
	signal := fmt.Sprintf("good_patterns=%d", len(goodFound))
	if len(badFound) > 0 {
		signal += fmt.Sprintf("; bad_patterns=%s", strings.Join(badFound, ","))
	}
	return model.EvalResult{
		Signal:  signal,
		Method:  "auto_rule",
		Pass:    len(badFound) == 0 && len(goodFound) >= 3,
		Details: strings.Join(badFound, ","),
	}
}
