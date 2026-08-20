package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"sync"
	"syscall"

	"github.com/monang404/luna-go/internal/permission"
)

type bgProcess struct {
	cmd  *exec.Cmd
	buf  bytes.Buffer
	mu   sync.Mutex
	done bool
	err  error
}

var (
	bgProcesses = make(map[string]*bgProcess)
	bgMu        sync.Mutex
	bgCounter   int
)

func startBackgroundProcess(cmd *exec.Cmd, name string) (Result, error) {
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return Result{}, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return Result{}, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return Result{}, fmt.Errorf("failed to start background process: %w", err)
	}

	bgMu.Lock()
	bgCounter++
	id := strconv.Itoa(bgCounter)
	bgp := &bgProcess{
		cmd: cmd,
	}
	bgProcesses[id] = bgp
	bgMu.Unlock()

	// Goroutine to capture output and monitor exit
	go func() {
		multi := io.MultiReader(stdoutPipe, stderrPipe)
		buf := make([]byte, 1024)
		for {
			n, err := multi.Read(buf)
			if n > 0 {
				bgp.mu.Lock()
				bgp.buf.Write(buf[:n])
				bgp.mu.Unlock()
			}
			if err != nil {
				break
			}
		}

		err := cmd.Wait()

		bgp.mu.Lock()
		bgp.done = true
		bgp.err = err
		bgp.mu.Unlock()
	}()

	return Result{Output: fmt.Errorf("Process started in background with ID: %s", id).Error()}, nil
}

// BashOutputTool reads the accumulated output of a background process.
type BashOutputTool struct{}

func (BashOutputTool) Name() string                      { return "bash_output" }
func (BashOutputTool) Capability() permission.Capability { return Registry["bash_output"].Capability }

func (BashOutputTool) Execute(_ context.Context, args json.RawMessage) (Result, error) {
	id := ExtractField(args, "id", "process_id")
	if id == "" {
		return Result{}, fmt.Errorf("ERROR: bash_output membutuhkan args.id")
	}

	bgMu.Lock()
	bgp, ok := bgProcesses[id]
	bgMu.Unlock()

	if !ok {
		return Result{}, fmt.Errorf("ERROR: proses dengan ID %s tidak ditemukan", id)
	}

	bgp.mu.Lock()
	defer bgp.mu.Unlock()

	out := bgp.buf.String()
	bgp.buf.Reset() // Clear buffer after reading

	if bgp.done {
		status := "selesai (exit 0)"
		if bgp.err != nil {
			status = fmt.Sprintf("gagal (%v)", bgp.err)
		}
		if out == "" {
			return Result{Output: fmt.Sprintf("Proses %s telah %s. Tidak ada output baru.", id, status)}, nil
		}
		return Result{Output: fmt.Sprintf("Proses %s telah %s.\nOutput akhir:\n%s", id, status, out)}, nil
	}

	if out == "" {
		return Result{Output: "(tidak ada output baru)"}, nil
	}
	return Result{Output: out}, nil
}

// KillShellTool kills a running background process.
type KillShellTool struct{}

func (KillShellTool) Name() string                      { return "kill_shell" }
func (KillShellTool) Capability() permission.Capability { return Registry["kill_shell"].Capability }

func (KillShellTool) Execute(_ context.Context, args json.RawMessage) (Result, error) {
	id := ExtractField(args, "id", "process_id")
	if id == "" {
		return Result{}, fmt.Errorf("ERROR: kill_shell membutuhkan args.id")
	}

	bgMu.Lock()
	bgp, ok := bgProcesses[id]
	bgMu.Unlock()

	if !ok {
		return Result{}, fmt.Errorf("ERROR: proses dengan ID %s tidak ditemukan", id)
	}

	bgp.mu.Lock()
	done := bgp.done
	bgp.mu.Unlock()

	if done {
		return Result{Output: fmt.Sprintf("Proses %s sudah selesai.", id)}, nil
	}

	// Send SIGTERM
	err := bgp.cmd.Process.Signal(syscall.SIGTERM)
	if err != nil {
		return Result{}, fmt.Errorf("ERROR: gagal mengirim SIGTERM ke proses %s: %w", id, err)
	}

	return Result{Output: fmt.Sprintf("Sinyal SIGTERM telah dikirim ke proses %s.", id)}, nil
}
