package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/monang404/luna-go/internal/permission"
)

// SubagentSpawner is a callback injected into the dispatcher to break import cycles.
type SubagentSpawner func(ctx context.Context, role string, goal string) (string, error)

type DelegateTaskTool struct {
	Spawner SubagentSpawner
}

func (t *DelegateTaskTool) Name() string {
	return "delegate_task"
}

func (t *DelegateTaskTool) Capability() permission.Capability {
	return permission.CapProcessExecute
}

func (t *DelegateTaskTool) Execute(ctx context.Context, args json.RawMessage) (Result, error) {
	if t.Spawner == nil {
		return Result{}, fmt.Errorf("delegate_task: Spawner is not configured")
	}

	var parsed struct {
		Role string `json:"role"`
		Goal string `json:"goal"`
	}
	if err := json.Unmarshal(args, &parsed); err != nil {
		return Result{}, fmt.Errorf("invalid arguments: %v", err)
	}

	if parsed.Role == "" || parsed.Goal == "" {
		return Result{}, fmt.Errorf("role and goal are required")
	}

	result, err := t.Spawner(ctx, parsed.Role, parsed.Goal)
	if err != nil {
		return Result{}, err
	}

	return Result{Output: result}, nil
}
