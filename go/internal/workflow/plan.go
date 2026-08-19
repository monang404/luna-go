package workflow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/monang404/luna-go/internal/aiops"
	"github.com/monang404/luna-go/internal/config"
)

// planSysPrompt mirrors aiplan's inline sysprompt.
const planSysPrompt = `Kamu expert perencana produktivitas. Diberikan sebuah goal, buat rencana terstruktur dalam format Markdown dengan bagian: 1) Ringkasan singkat goal (2 kalimat). 2) Breakdown milestone (fase-fase besar, urut). 3) Checklist task per milestone pakai format '- [ ] task', konkret dan actionable. 4) Estimasi waktu tiap milestone kalau relevan. 5) Potensi hambatan & cara mitigasinya. Bahasa Indonesia, langsung ke inti, tanpa basa-basi pembuka.`

// ErrPlanUsage mirrors aiplan's "Usage: luna plan <goal/tujuan>" message.
var ErrPlanUsage = errors.New("workflow: usage: goal is required")

// PlanResult is Plan's return shape.
type PlanResult struct {
	Content string
	Outfile string
}

// Plan mirrors aiplan(goal): generate a structured Markdown plan and
// save it under AI_PLAN_DIR/<slug>_<ts>.md.
func (s *Service) Plan(ctx context.Context, goal string) (PlanResult, error) {
	if err := needAnyKey(config.TaskProviderOrder); err != nil {
		return PlanResult{}, err
	}
	if goal == "" {
		return PlanResult{}, ErrPlanUsage
	}
	if err := os.MkdirAll(s.Paths.PlanDir, 0o755); err != nil {
		return PlanResult{}, err
	}

	res, err := s.Requester.Complete(ctx, planSysPrompt, "Goal: "+goal, config.TaskSmart, config.TaskProviderOrder, 0)
	if err != nil || res.Content == "" {
		return PlanResult{}, fmt.Errorf("workflow: plan generation failed: %w", err)
	}

	outfile := filepath.Join(s.Paths.PlanDir, fmt.Sprintf("%s_%s.md", aiops.Slugify(goal, 40), aiops.Timestamp()))
	if err := os.WriteFile(outfile, []byte(res.Content+"\n"), 0o644); err != nil {
		return PlanResult{}, err
	}
	return PlanResult{Content: res.Content, Outfile: outfile}, nil
}
