package memory

import (
	"harness/internal/model"
	"strings"
)

type Trajectory struct {
	Run         *model.Run
	Prompt      *model.Prompt
	Steps       []Step
	Outcome     string
}

type Step struct {
	Action  string
	Result  string
	Success bool
}

type ExtractedSkill struct {
	Name         string
	TriggerWords []string
	Steps        []Step
	SuccessRate  float64
	Version      int
}

type Memory struct {
	UniversalMemory  []string
	UserPreferences  map[string]string
	SkillVersions    map[string]int
}

func ExtractTrajectory(run *model.Run, prompt *model.Prompt) *Trajectory {
	t := &Trajectory{
		Run:    run,
		Prompt: prompt,
	}

	if run.Status == "ok" {
		t.Outcome = "success"
	} else if run.Status == "partial" {
		t.Outcome = "partial_success"
	} else {
		t.Outcome = "failure"
	}

	if run.Feedback != "" {
		t.Steps = append(t.Steps, Step{
			Action:  "user_feedback",
			Result:  run.Feedback,
			Success: false,
		})
	}
	if run.Correction != "" {
		t.Steps = append(t.Steps, Step{
			Action:  "correction_applied",
			Result:  run.Correction,
			Success: true,
		})
	}

	return t
}

func ExtractSkill(trajectory *Trajectory) *ExtractedSkill {
	skill := &ExtractedSkill{
		Name:    trajectory.Run.Task,
		Version: 1,
	}

	words := strings.Fields(trajectory.Run.Task)
	triggerSet := map[string]bool{}
	for _, w := range words {
		w = strings.ToLower(strings.TrimSpace(w))
		if len(w) > 1 {
			triggerSet[w] = true
		}
	}
	for w := range triggerSet {
		skill.TriggerWords = append(skill.TriggerWords, w)
	}

	for _, step := range trajectory.Steps {
		skill.Steps = append(skill.Steps, step)
	}

	if trajectory.Outcome == "success" {
		skill.SuccessRate = 1.0
	} else if trajectory.Outcome == "partial_success" {
		skill.SuccessRate = 0.5
	} else {
		skill.SuccessRate = 0.0
	}

	return skill
}

func ShouldTriggerLearning(run *model.Run) bool {
	if run.Status == "fail" {
		return true
	}
	if run.Status == "partial" && run.Feedback != "" {
		return true
	}
	return false
}

func ShouldDisableSkill(consecutiveFails int) bool {
	return consecutiveFails >= 3
}

func UpdateMemory(mem *Memory, run *model.Run) *Memory {
	if mem == nil {
		mem = &Memory{
			UserPreferences: map[string]string{},
			SkillVersions:   map[string]int{},
		}
	}

	if run.Correction != "" {
		mem.UniversalMemory = append(mem.UniversalMemory, run.Correction)
	}

	if run.EvalSignal != "" {
		if strings.Contains(run.EvalSignal, "missing=") {
			parts := strings.SplitN(run.EvalSignal, "missing=", 2)
			if len(parts) == 2 {
				mem.UserPreferences["last_missing"] = parts[1]
			}
		}
	}

	return mem
}
