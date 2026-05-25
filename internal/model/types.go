package model

type Prompt struct {
	RecordID      string `json:"record_id"`
	Name          string `json:"name"`
	Category      string `json:"category"`
	Template      string `json:"template"`
	Version       int    `json:"version"`
	Status        string `json:"status"`
	StrategyLevel string `json:"strategy_level"`
	CoverageReport string `json:"coverage_report"`
	Notes         string `json:"notes"`
}

type Run struct {
	RecordID    string `json:"record_id"`
	PromptID    string `json:"prompt_id"`
	Task        string `json:"task"`
	Output      string `json:"output_summary"`
	Status      string `json:"status"`
	EvalSignal  string `json:"eval_signal"`
	EvalMethod  string `json:"eval_method"`
	Feedback    string `json:"feedback"`
	Correction  string `json:"correction"`
	Iteration   int    `json:"iteration"`
	NextPromptID string `json:"next_prompt_id"`
	Convergence string `json:"convergence"`
	CreatedAt   string `json:"created_at"`
}

type EvalResult struct {
	Signal    string `json:"signal"`
	Method    string `json:"method"`
	Pass      bool   `json:"pass"`
	Details   string `json:"details,omitempty"`
}

type ConvergenceStatus string

const (
	Converging ConvergenceStatus = "converging"
	Stuck      ConvergenceStatus = "stuck"
	Converged  ConvergenceStatus = "converged"
)

type StrategyLevel string

const (
	Basic       StrategyLevel = "basic"
	Constrained StrategyLevel = "constrained"
	FewShot     StrategyLevel = "few_shot"
	Strict      StrategyLevel = "strict"
)

var StrategyLevels = []StrategyLevel{Basic, Constrained, FewShot, Strict}

func (s StrategyLevel) Next() StrategyLevel {
	for i, level := range StrategyLevels {
		if level == s && i+1 < len(StrategyLevels) {
			return StrategyLevels[i+1]
		}
	}
	return s
}

func (s StrategyLevel) IsMax() bool {
	return s == Strict
}

func (s StrategyLevel) UpgradeHint() string {
	switch s {
	case Basic:
		return "增加场景枚举约束、格式要求、边界条件"
	case Constrained:
		return "增加 2-3 个高质量 few-shot 示例"
	case FewShot:
		return "增加反例、输出格式校验规则、禁止项清单"
	default:
		return "已是最高层级，建议人工审查或拆分任务"
	}
}
