package models

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
)

// fakePython returns a PythonInterpreter pointing at the current test
// binary, configured (via env vars) to behave as a fake Python
// interpreter, since CI can't assume Python or huggingface_hub is
// installed. Unlike runtime/process_test.go's fake-subprocess seam (which
// selects its helper via "-test.run=^TestHelperProcess$", appended to a
// caller-controlled ExtraArgs slice), Pull/Delete build their subprocess
// argv internally as "-c <snippet> <modelID>" with no room for an extra
// flag, so this package can't reuse that exact seam — instead TestMain
// below intercepts the re-exec'd process before `go test`'s own flag
// parsing/test selection ever runs, sidestepping the need for -test.run
// entirely.
func fakePython(t *testing.T, mode string) PythonInterpreter {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}
	t.Setenv("GO_WANT_HELPER_PROCESS", "1")
	t.Setenv("HELPER_MODE", mode)
	return PythonInterpreter(exe)
}

// TestMain intercepts the re-exec'd fake-Python-interpreter subprocess
// (see fakePython) before the testing package parses argv as test flags,
// since that argv is "-c <snippet> <modelID>", not `go test` flags.
func TestMain(m *testing.M) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") == "1" {
		runFakePythonInterpreter()
		return
	}
	os.Exit(m.Run())
}

// runFakePythonInterpreter stands in for the Python interpreter,
// selected by HELPER_MODE, printing the canned stdout/exit behavior each
// Pull/Delete test case expects.
func runFakePythonInterpreter() {
	switch os.Getenv("HELPER_MODE") {
	case "pull_ok":
		fmt.Println(`{"id":"org/model","size_bytes":123456}`)
	case "pull_no_hub":
		fmt.Println(`{"error":"huggingface_hub is not importable"}`)
	case "pull_error":
		fmt.Println(`{"error":"connection refused"}`)
	case "delete_ok":
		fmt.Println(`{"id":"org/model","freed_bytes":98765}`)
	case "delete_not_found":
		fmt.Println(`{"error":"no_cached_model"}`)
	case "delete_no_hub":
		fmt.Println(`{"error":"huggingface_hub is not importable"}`)
	case "malformed":
		fmt.Println(`not-json`)
	case "exit_nonzero":
		fmt.Fprintln(os.Stderr, "boom")
		os.Exit(1)
	}
	os.Exit(0)
}

func TestPullSuccess(t *testing.T) {
	python := fakePython(t, "pull_ok")
	result, err := Pull(context.Background(), python, "org/model")
	if err != nil {
		t.Fatalf("Pull() error = %v", err)
	}
	if result.ID != "org/model" || result.SizeBytes != 123456 {
		t.Errorf("Pull() = %+v", result)
	}
}

func TestPullHuggingFaceHubUnavailable(t *testing.T) {
	python := fakePython(t, "pull_no_hub")
	_, err := Pull(context.Background(), python, "org/model")
	if !errors.Is(err, ErrHuggingFaceHubUnavailable) {
		t.Fatalf("Pull() error = %v, want ErrHuggingFaceHubUnavailable", err)
	}
}

func TestPullGenericError(t *testing.T) {
	python := fakePython(t, "pull_error")
	_, err := Pull(context.Background(), python, "org/model")
	if err == nil {
		t.Fatal("expected error")
	}
	if errors.Is(err, ErrHuggingFaceHubUnavailable) {
		t.Error("expected non-hub error, got ErrHuggingFaceHubUnavailable")
	}
}

func TestPullRejectsEmptyModelID(t *testing.T) {
	_, err := Pull(context.Background(), PythonInterpreter("python3"), "")
	if err == nil {
		t.Fatal("expected error for empty model id")
	}
}

func TestPullMalformedOutput(t *testing.T) {
	python := fakePython(t, "malformed")
	_, err := Pull(context.Background(), python, "org/model")
	if err == nil {
		t.Fatal("expected error for malformed JSON output")
	}
}

func TestPullSubprocessFailure(t *testing.T) {
	python := fakePython(t, "exit_nonzero")
	_, err := Pull(context.Background(), python, "org/model")
	if err == nil {
		t.Fatal("expected error for non-zero exit")
	}
}

func TestDeleteSuccess(t *testing.T) {
	python := fakePython(t, "delete_ok")
	result, err := Delete(context.Background(), python, "org/model")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if result.ID != "org/model" || result.FreedBytes != 98765 {
		t.Errorf("Delete() = %+v", result)
	}
}

func TestDeleteNotFound(t *testing.T) {
	python := fakePython(t, "delete_not_found")
	_, err := Delete(context.Background(), python, "org/model")
	if !errors.Is(err, ErrLocalModelNotFound) {
		t.Fatalf("Delete() error = %v, want ErrLocalModelNotFound", err)
	}
}

func TestDeleteHuggingFaceHubUnavailable(t *testing.T) {
	python := fakePython(t, "delete_no_hub")
	_, err := Delete(context.Background(), python, "org/model")
	if !errors.Is(err, ErrHuggingFaceHubUnavailable) {
		t.Fatalf("Delete() error = %v, want ErrHuggingFaceHubUnavailable", err)
	}
}

func TestDeleteRejectsEmptyModelID(t *testing.T) {
	_, err := Delete(context.Background(), PythonInterpreter("python3"), "")
	if err == nil {
		t.Fatal("expected error for empty model id")
	}
}

func TestResolvePythonInterpreter(t *testing.T) {
	if got := ResolvePythonInterpreter("/custom/python"); got != "/custom/python" {
		t.Errorf("ResolvePythonInterpreter(explicit) = %q", got)
	}

	t.Setenv("VELOXQUANT_PYTHON", "/env/python")
	if got := ResolvePythonInterpreter(""); got != "/env/python" {
		t.Errorf("ResolvePythonInterpreter(env) = %q", got)
	}

	t.Setenv("VELOXQUANT_PYTHON", "")
	if got := ResolvePythonInterpreter(""); got != "python3" {
		t.Errorf("ResolvePythonInterpreter(default) = %q, want python3", got)
	}
}
