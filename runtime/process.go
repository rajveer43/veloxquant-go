package runtime

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// readyPrefix is the line the veloxquant CLI's "serve" subcommand prints to
// stdout, immediately before it binds its listen port, once the model is
// loaded and the KV cache is wired in. See VeloxQuant-MLX's
// veloxquant_mlx/cli/serve.py (emit_ready). Waiting for this handshake
// instead of blindly polling /health means Process.Start doesn't report
// "ready" while a slow model load is still in progress.
const readyPrefix = "VELOXQUANT_READY "

// DefaultCommand is the binary Process launches by default: the
// `veloxquant` CLI installed by the VeloxQuant-MLX Python package
// (`pip install veloxquant-mlx`).
const DefaultCommand = "veloxquant"

// ReadyInfo is the machine-readable handshake the runtime process prints to
// stdout once it's ready to accept requests.
type ReadyInfo struct {
	SchemaVersion int    `json:"schema_version"`
	Model         string `json:"model"`
	Method        string `json:"method"`
	Bits          int    `json:"bits"`
	Host          string `json:"host"`
	Port          int    `json:"port"`

	Endpoints struct {
		OpenAIBaseURL   string `json:"openai_base_url"`
		ChatCompletions string `json:"chat_completions"`
		Completions     string `json:"completions"`
		Models          string `json:"models"`
	} `json:"endpoints"`

	AccountingOnly bool   `json:"accounting_only"`
	AccountingNote string `json:"accounting_note"`
}

// ProcessConfig configures a local VeloxQuant runtime process launched by
// StartProcess.
type ProcessConfig struct {
	// Command is the binary to run. Defaults to DefaultCommand ("veloxquant").
	Command string

	// Model is the Hugging Face model id or local path to serve. Required.
	Model string

	// Method is the KV-cache compression method (e.g. "kivi"). Empty uses
	// the runtime's own default.
	Method string

	// Host is the listen address. Defaults to "127.0.0.1".
	Host string

	// Port is the listen port. Defaults to 8000 (the veloxquant CLI's own
	// default), independent of DefaultURL used by Client.
	Port int

	// ExtraArgs are appended verbatim to the command line, for flags this
	// SDK doesn't model directly (e.g. "--bits", "3").
	ExtraArgs []string

	// Stdout and Stderr, if set, receive the subprocess's output streams
	// after the ready handshake line has been consumed from stdout. If
	// nil, that output is discarded.
	Stdout io.Writer
	Stderr io.Writer

	// ReadyTimeout bounds how long Start waits for the ready handshake
	// before giving up and killing the process. Defaults to 5 minutes,
	// since loading a large model can take a while.
	ReadyTimeout time.Duration
}

func (c ProcessConfig) command() string {
	if c.Command != "" {
		return c.Command
	}
	return DefaultCommand
}

func (c ProcessConfig) host() string {
	if c.Host != "" {
		return c.Host
	}
	return "127.0.0.1"
}

func (c ProcessConfig) port() int {
	if c.Port != 0 {
		return c.Port
	}
	return 8000
}

func (c ProcessConfig) readyTimeout() time.Duration {
	if c.ReadyTimeout > 0 {
		return c.ReadyTimeout
	}
	return 5 * time.Minute
}

func (c ProcessConfig) args() []string {
	args := []string{"serve", "--model", c.Model, "--host", c.host(), "--port", strconv.Itoa(c.port())}
	if c.Method != "" {
		args = append(args, "--method", c.Method)
	}
	args = append(args, c.ExtraArgs...)
	return args
}

// Process manages the lifecycle of a local VeloxQuant runtime subprocess:
// starting it, waiting for it to report readiness, and stopping it
// cleanly. Use StartProcess to construct one; the zero value is not
// usable.
type Process struct {
	cfg ProcessConfig
	cmd *exec.Cmd

	mu   sync.Mutex
	done chan struct{}
	err  error
}

// ErrModelRequired is returned by StartProcess when ProcessConfig.Model is
// empty.
var ErrModelRequired = errors.New("runtime: process config requires a model")

// StartProcess launches a local VeloxQuant runtime process per cfg and
// blocks until it reports readiness (via its stdout handshake), the
// process exits, cfg.ReadyTimeout elapses, or ctx is canceled — whichever
// happens first. On success it returns a Process handle whose URL matches
// where the runtime is now listening.
//
// The returned Process's subprocess keeps running after StartProcess
// returns; call Stop when done with it.
func StartProcess(ctx context.Context, cfg ProcessConfig) (*Process, error) {
	if cfg.Model == "" {
		return nil, ErrModelRequired
	}

	cmd := exec.Command(cfg.command(), cfg.args()...)
	cmd.Stdin = nil

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("runtime process: stdout pipe: %w", err)
	}
	if cfg.Stderr != nil {
		cmd.Stderr = cfg.Stderr
	} else {
		cmd.Stderr = os.Stderr
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("runtime process: start %s: %w", cfg.command(), err)
	}

	p := &Process{cfg: cfg, cmd: cmd, done: make(chan struct{})}

	readyCh := make(chan ReadyInfo, 1)
	scanErrCh := make(chan error, 1)
	go p.scanStdout(stdout, readyCh, scanErrCh)
	go p.wait()

	readyCtx, cancel := context.WithTimeout(ctx, cfg.readyTimeout())
	defer cancel()

	select {
	case info := <-readyCh:
		p.mu.Lock()
		p.cfg.Host = info.Host
		p.cfg.Port = info.Port
		p.mu.Unlock()
		return p, nil
	case err := <-scanErrCh:
		_ = p.Stop(context.Background())
		return nil, fmt.Errorf("runtime process: %w", err)
	case <-p.done:
		p.mu.Lock()
		exitErr := p.err
		p.mu.Unlock()
		return nil, fmt.Errorf("runtime process: exited before becoming ready: %w", exitErr)
	case <-readyCtx.Done():
		_ = p.Stop(context.Background())
		return nil, fmt.Errorf("runtime process: %w waiting for ready handshake", readyCtx.Err())
	}
}

func (p *Process) scanStdout(stdout io.ReadCloser, readyCh chan<- ReadyInfo, errCh chan<- error) {
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	sawReady := false
	for scanner.Scan() {
		line := scanner.Text()
		if !sawReady {
			if payload, ok := strings.CutPrefix(line, readyPrefix); ok {
				var info ReadyInfo
				if err := json.Unmarshal([]byte(payload), &info); err != nil {
					errCh <- fmt.Errorf("decode ready handshake: %w", err)
					return
				}
				sawReady = true
				readyCh <- info
				continue
			}
		}
		if p.cfg.Stdout != nil {
			fmt.Fprintln(p.cfg.Stdout, line)
		}
	}
	if err := scanner.Err(); err != nil {
		errCh <- fmt.Errorf("read stdout: %w", err)
	}
}

func (p *Process) wait() {
	err := p.cmd.Wait()
	p.mu.Lock()
	p.err = err
	p.mu.Unlock()
	close(p.done)
}

// URL returns the base URL the runtime process is listening on, once
// StartProcess has returned successfully.
func (p *Process) URL() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return fmt.Sprintf("http://%s:%d", p.cfg.host(), p.cfg.port())
}

// Wait blocks until the process exits, returning its exit error (if any).
func (p *Process) Wait() error {
	<-p.done
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.err
}

// Done returns a channel that is closed when the process exits.
func (p *Process) Done() <-chan struct{} {
	return p.done
}

// Stop signals the runtime process to shut down (SIGTERM, then SIGKILL if
// it doesn't exit before ctx is done) and waits for it to exit. Safe to
// call multiple times.
func (p *Process) Stop(ctx context.Context) error {
	select {
	case <-p.done:
		return nil
	default:
	}

	if err := terminate(p.cmd.Process); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("runtime process: signal: %w", err)
	}

	select {
	case <-p.done:
		return nil
	case <-ctx.Done():
		_ = p.cmd.Process.Kill()
		<-p.done
		return ctx.Err()
	}
}
