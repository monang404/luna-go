package aiops

import (
	"context"
	"os"
	"testing"

	"github.com/monang404/luna-go/internal/config"
)

func TestComplete_NoProviderConfigured(t *testing.T) {
	// Ensure no provider API key env vars are set for this test.
	for _, v := range []string{"GROQ_API_KEY", "GEMINI_API_KEY", "CEREBRAS_API_KEY", "DEEPSEEK_API_KEY"} {
		old, had := os.LookupEnv(v)
		os.Unsetenv(v)
		if had {
			defer os.Setenv(v, old)
		}
	}

	r := NewRequester()
	_, err := r.Complete(context.Background(), "system", "hello", config.TaskFast, config.TaskProviderOrderFast, 0)
	if err == nil {
		t.Fatal("expected an error when no provider has an API key configured")
	}
}

func TestComplete_EmptyOrderList(t *testing.T) {
	r := NewRequester()
	_, err := r.Complete(context.Background(), "system", "hello", config.TaskFast, nil, 0)
	if err == nil {
		t.Fatal("expected an error for an empty provider order")
	}
}
