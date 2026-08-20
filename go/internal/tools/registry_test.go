package tools

import (
	"os"
	"strings"
	"testing"
)

func TestManifest_RunCommandHiddenByDefault(t *testing.T) {
	os.Unsetenv("AI_AGENT_EXPOSE_ARBITRARY_SHELL")
	if strings.Contains(Manifest(), "run_command | capability") {
		t.Error("run_command tidak boleh muncul di manifest tanpa AI_AGENT_EXPOSE_ARBITRARY_SHELL=1")
	}

	os.Setenv("AI_AGENT_EXPOSE_ARBITRARY_SHELL", "1")
	defer os.Unsetenv("AI_AGENT_EXPOSE_ARBITRARY_SHELL")
	if !strings.Contains(Manifest(), "run_command | capability") {
		t.Error("run_command harus muncul di manifest jika AI_AGENT_EXPOSE_ARBITRARY_SHELL=1")
	}
}
