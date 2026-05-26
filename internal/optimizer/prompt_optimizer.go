package optimizer

import (
	"fmt"
	"harness/internal/model"
	"regexp"
	"strings"
)

type PromptSection struct {
	ID      int
	Header  string
	Content string
}

type BadCaseAttribution struct {
	SectionID   int
	Reason      string
	Expectation string
}

type OptimizationCandidate struct {
	SectionID int
	Strategy  string
	Content   string
}

type GSBResult struct {
	Good int
	Same int
	Bad  int
}

func (g GSBResult) Score() int {
	return g.Good - g.Bad
}

func (g GSBResult) IsBetter() bool {
	return g.Score() > 0
}

func SplitPrompt(template string) []PromptSection {
	blocks := regexp.MustCompile(`\n{2,}`).Split(template, -1)
	headerRe := regexp.MustCompile(`^#{1,3}\s+`)

	var sections []PromptSection
	id := 1
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		header := ""
		lines := strings.SplitN(block, "\n", 2)
		if headerRe.MatchString(lines[0]) {
			header = strings.TrimSpace(headerRe.ReplaceAllString(lines[0], ""))
		}
		sections = append(sections, PromptSection{
			ID:      id,
			Header:  header,
			Content: block,
		})
		id++
	}
	return sections
}

func AttributeBadCase(sections []PromptSection, run *model.Run) []BadCaseAttribution {
	var attributions []BadCaseAttribution
	keywords := map[int][]string{}

	for _, kw := range []string{"场景", "覆盖", "格式", "选择器", "等待", "架构", "断言", "约束", "模板", "输出"} {
		for i, s := range sections {
			if strings.Contains(s.Content, kw) || strings.Contains(s.Header, kw) {
				keywords[i] = append(keywords[i], kw)
			}
		}
	}

	feedback := run.Feedback + " " + run.Correction
	for kw, sectionIDs := range map[string][]int{} {
		_ = kw
		_ = sectionIDs
	}

	if strings.Contains(feedback, "场景") || strings.Contains(feedback, "覆盖") {
		for i, s := range sections {
			if strings.Contains(strings.ToLower(s.Header), "skill") ||
				strings.Contains(s.Header, "技能") ||
				strings.Contains(s.Header, "约束") ||
				strings.Contains(s.Header, "constraint") {
				attributions = append(attributions, BadCaseAttribution{
					SectionID:   s.ID,
					Reason:      fmt.Sprintf("反馈涉及场景覆盖，归因到区块%d(%s)", s.ID, s.Header),
					Expectation: "需要在该区块增加场景枚举约束",
				})
			}
			_ = i
		}
	}

	if strings.Contains(feedback, "选择器") || strings.Contains(feedback, "定位") {
		for _, s := range sections {
			if strings.Contains(strings.ToLower(s.Header), "constraint") ||
				strings.Contains(s.Header, "约束") ||
				strings.Contains(s.Header, "规范") {
				attributions = append(attributions, BadCaseAttribution{
					SectionID:   s.ID,
					Reason:      fmt.Sprintf("反馈涉及选择器策略，归因到区块%d(%s)", s.ID, s.Header),
					Expectation: "需要在该区块指定选择器策略",
				})
			}
		}
	}

	if len(attributions) == 0 {
		if len(sections) > 0 {
			last := sections[len(sections)-1]
			attributions = append(attributions, BadCaseAttribution{
				SectionID:   last.ID,
				Reason:      fmt.Sprintf("未精确归因，默认归因到末尾区块%d(%s)", last.ID, last.Header),
				Expectation: "根据反馈内容调整该区块",
			})
		}
	}

	return attributions
}

func GenerateOptimizations(sections []PromptSection, attributions []BadCaseAttribution, currentLevel model.StrategyLevel) []OptimizationCandidate {
	var candidates []OptimizationCandidate
	attributedSections := map[int]bool{}
	for _, a := range attributions {
		attributedSections[a.SectionID] = true
	}

	for _, s := range sections {
		if !attributedSections[s.ID] {
			continue
		}

		candidates = append(candidates, OptimizationCandidate{
			SectionID: s.ID,
			Strategy:  "constrain",
			Content:   s.Content + "\n- 必须覆盖所有要求的场景类型\n- 输出必须包含完整的字段",
		})

		if currentLevel == model.Constrained || currentLevel == model.FewShot || currentLevel == model.Strict {
			candidates = append(candidates, OptimizationCandidate{
				SectionID: s.ID,
				Strategy:  "few_shot",
				Content:   s.Content + "\n\n示例：\n输入：登录功能\n输出：\n| 用例标题 | 优先级 | 前置条件 | 操作步骤 | 预期结果 |\n| 正常登录 | P0 | 账号已注册 | 输入正确账号密码点击登录 | 登录成功跳转首页 |",
			})
		}

		if currentLevel == model.Strict {
			candidates = append(candidates, OptimizationCandidate{
				SectionID: s.ID,
				Strategy:  "strict",
				Content:   s.Content + "\n\n禁止项：\n- 禁止使用 data-testid 定位\n- 禁止使用 sleep() 等待\n- 禁止省略断言",
			})
		}
	}

	return candidates
}

func EvaluateGSB(original string, optimized string, evalResults []bool) GSBResult {
	var result GSBResult
	for _, pass := range evalResults {
		if pass {
			result.Good++
		} else {
			result.Bad++
		}
	}
	if result.Good == 0 && result.Bad == 0 {
		result.Same = 1
	}
	return result
}

func ReconstructPrompt(sections []PromptSection, sectionID int, newContent string) string {
	var parts []string
	for _, s := range sections {
		if s.ID == sectionID {
			parts = append(parts, newContent)
		} else {
			parts = append(parts, s.Content)
		}
	}
	return strings.Join(parts, "\n\n")
}
