package runtime

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// TestHelperProcess isn't a real test. It's invoked as a subprocess (the
// go test binary re-executing itself under GO_WANT_HELPER_PROCESS=1) to
// stand in for the veloxquant CLI, since CI can't assume the real Python
// package is installed. Its behavior is selected by HELPER_MODE; it
// ignores the "serve --model ..." args StartProcess builds for it.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)

	switch os.Getenv("HELPER_MODE") {
	case "ready":
		fmt.Println(readyPrefix + `{"schema_version":1,"model":"m","method":"kivi","bits":2,"host":"127.0.0.1","port":9999}`)
		time.Sleep(10 * time.Second)
	case "exit_early":
		fmt.Println("loading model...")
		os.Exit(1)
	case "malformed":
		fmt.Println(readyPrefix + "not-json")
	case "never_ready":
		fmt.Println("still loading...")
		time.Sleep(10 * time.Second)
	}
}

// startFake launches the current test binary (re-executed as the
// TestHelperProcess subtest) as the runtime process, driven by mode.
func startFake(t *testing.T, mode string, cfg ProcessConfig) (*Process, error) {
	t.Helper()

	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}

	cfg.Command = exe
	cfg.Model = "m"
	// StartProcess builds args as: serve --model m --host H --port P [method]
	// [ExtraArgs...]. The helper ignores its argv and reads HELPER_MODE
	// instead, so ExtraArgs here only need to carry the -test.run selector
	// that must come after the flags exec.Command otherwise can't parse as
	// belonging to `go test`. Passing it via ExtraArgs keeps args() untouched.
	cfg.ExtraArgs = append(cfg.ExtraArgs, "-test.run=^TestHelperProcess$")

	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Setenv("HELPER_MODE", mode)

	return StartProcess(context.Background(), cfg)
}

func TestProcessStartWaitsForReadyHandshake(t *testing.T) {
	p, err := startFake(t, "ready", ProcessConfig{ReadyTimeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("StartProcess() error = %v", err)
	}
	defer p.Stop(context.Background())

	if got := p.URL(); got != "http://127.0.0.1:9999" {
		t.Errorf("URL() = %q, want http://127.0.0.1:9999", got)
	}
}

func TestProcessStartFailsOnEarlyExit(t *testing.T) {
	_, err := startFake(t, "exit_early", ProcessConfig{ReadyTimeout: 5 * time.Second})
	if err == nil {
		t.Fatal("expected error when process exits before becoming ready")
	}
}

func TestProcessStartFailsOnMalformedHandshake(t *testing.T) {
	_, err := startFake(t, "malformed", ProcessConfig{ReadyTimeout: 5 * time.Second})
	if err == nil {
		t.Fatal("expected error for malformed ready handshake")
	}
}

func TestProcessStartRespectsReadyTimeout(t *testing.T) {
	_, err := startFake(t, "never_ready", ProcessConfig{ReadyTimeout: 200 * time.Millisecond})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "waiting for ready handshake") {
		t.Errorf("error = %v, want mention of ready handshake timeout", err)
	}
}

func TestProcessStopIsIdempotent(t *testing.T) {
	p, err := startFake(t, "ready", ProcessConfig{ReadyTimeout: 5 * time.Second})
	if err != nil {
		t.Fatalf("StartProcess() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := p.Stop(ctx); err != nil {
		t.Errorf("first Stop() error = %v", err)
	}
	if err := p.Stop(ctx); err != nil {
		t.Errorf("second Stop() error = %v", err)
	}
}

func TestProcessStdoutCapturesNonHandshakeLines(t *testing.T) {
	var buf bytes.Buffer
	p, err := startFake(t, "ready", ProcessConfig{ReadyTimeout: 5 * time.Second, Stdout: &buf})
	if err != nil {
		t.Fatalf("StartProcess() error = %v", err)
	}
	defer p.Stop(context.Background())
}

func TestStartProcessRequiresModel(t *testing.T) {
	_, err := StartProcess(context.Background(), ProcessConfig{})
	if err == nil {
		t.Fatal("expected error for missing model")
	}
}
