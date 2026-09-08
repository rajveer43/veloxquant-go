package models

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// PythonInterpreter is the path to (or name of) the Python interpreter
// Pull and Delete invoke to talk to huggingface_hub. Construct one
// directly if you already know the interpreter path, or use
// ResolvePythonInterpreter to apply this package's default resolution
// order.
type PythonInterpreter string

// ResolvePythonInterpreter picks the Python interpreter Pull/Delete should
// use: explicit if non-empty, otherwise the VELOXQUANT_PYTHON environment
// variable, otherwise "python3" — matching the resolution order the TS
// SDK's resolveInterpreter() uses (src/python/interpreter.ts), so a
// VELOXQUANT_PYTHON set for one sibling SDK works for this one too.
func ResolvePythonInterpreter(explicit string) PythonInterpreter {
	if explicit != "" {
		return PythonInterpreter(explicit)
	}
	if envPath := os.Getenv("VELOXQUANT_PYTHON"); envPath != "" {
		return PythonInterpreter(envPath)
	}
	return "python3"
}

// ErrHuggingFaceHubUnavailable is returned by Pull and Delete when the
// configured Python interpreter cannot import huggingface_hub. Distinct
// from a generic pull/delete failure so callers can errors.Is against it
// specifically (e.g. to print a "pip install huggingface_hub" hint).
var ErrHuggingFaceHubUnavailable = errors.New("huggingface_hub is not importable")

// ErrLocalModelNotFound is returned by Delete when modelID has no cached
// revisions to remove.
var ErrLocalModelNotFound = errors.New("no cached model found with that id")

// PullResult is the result of a successful Pull.
type PullResult struct {
	ID        string
	SizeBytes uint64
}

// DeleteResult is the result of a successful Delete.
type DeleteResult struct {
	ID         string
	FreedBytes uint64
}

// pullSnippet downloads modelID's weights into the local Hugging Face
// cache via snapshot_download(), then reports its size on disk via
// scan_cache_dir() — the same library ScanLocal's TS sibling reads back
// from. Structural equivalent of localModels.ts:106-128.
const pullSnippet = `
import json, sys
try:
    from huggingface_hub import snapshot_download, scan_cache_dir
    from huggingface_hub.errors import CacheNotFound
except ImportError:
    print(json.dumps({"error": "huggingface_hub is not importable"}))
else:
    model_id = sys.argv[1]
    try:
        snapshot_download(repo_id=model_id)
    except Exception as e:
        print(json.dumps({"error": str(e)}))
    else:
        try:
            info = scan_cache_dir()
        except CacheNotFound:
            size_bytes = 0
        else:
            repo = next((r for r in info.repos if r.repo_type == "model" and r.repo_id == model_id), None)
            size_bytes = repo.size_on_disk if repo is not None else 0
        print(json.dumps({"id": model_id, "size_bytes": size_bytes}))
`

// deleteSnippet removes modelID's weights from the local Hugging Face
// cache using huggingface_hub's own eviction API (scan -> delete_revisions
// -> execute) rather than an rm -rf on a resolved path: the cache's blob
// layout is content-addressed and shared across revisions/repos via
// symlinks, so a naive recursive delete risks corrupting a different
// cached model's blobs. Structural equivalent of localModels.ts:165-187.
const deleteSnippet = `
import json, sys
try:
    from huggingface_hub import scan_cache_dir
    from huggingface_hub.errors import CacheNotFound
except ImportError:
    print(json.dumps({"error": "huggingface_hub is not importable"}))
else:
    model_id = sys.argv[1]
    try:
        info = scan_cache_dir()
    except CacheNotFound:
        print(json.dumps({"error": "no_cached_model"}))
    else:
        repo = next((r for r in info.repos if r.repo_type == "model" and r.repo_id == model_id), None)
        if repo is None:
            print(json.dumps({"error": "no_cached_model"}))
        else:
            revisions = [rev.commit_hash for rev in repo.revisions]
            strategy = info.delete_revisions(*revisions)
            strategy.execute()
            print(json.dumps({"id": model_id, "freed_bytes": strategy.expected_freed_size}))
`

type snippetResult struct {
	ID         string `json:"id"`
	SizeBytes  uint64 `json:"size_bytes"`
	FreedBytes uint64 `json:"freed_bytes"`
	Error      string `json:"error"`
}

// runPythonSnippet invokes `python -c snippet modelID`, with modelID
// passed as its own argv element (never interpolated into the snippet
// string), so a model id containing shell or Python metacharacters cannot
// be interpreted as code — matching localModels.ts's command-injection
// mitigation. Returns stdout on success (exit code 0); a non-zero exit is
// reported via stderr if present, or the exec error otherwise.
func runPythonSnippet(ctx context.Context, python PythonInterpreter, snippet, modelID string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, string(python), "-c", snippet, modelID)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if msg := stderr.String(); msg != "" {
			return nil, errors.New(msg)
		}
		return nil, err
	}
	return stdout.Bytes(), nil
}

// Pull downloads modelID's weights into the local Hugging Face cache (see
// LocalCacheDir) via huggingface_hub's snapshot_download(), without
// loading the model into MLX or starting a runtime process.
//
// Unlike ScanLocal, which is a dependency-free filesystem walk, Pull
// shells out to python to run huggingface_hub's own download/cache logic:
// resolving a model id to its file manifest, handling partial/resumable
// downloads, and content-addressing new blobs against the existing cache
// is a large, fragile undertaking to reimplement natively in Go against a
// cache format Go doesn't own. python must have huggingface_hub
// importable; see ErrHuggingFaceHubUnavailable.
//
// modelID is passed to the subprocess as its own argv element, never
// interpolated into the Python source, so it cannot be used to inject
// shell or Python syntax.
//
// There is no default timeout: downloads can take many minutes for large
// models, so ctx alone governs cancellation.
func Pull(ctx context.Context, python PythonInterpreter, modelID string) (PullResult, error) {
	if modelID == "" {
		return PullResult{}, fmt.Errorf("pull model: model id must not be empty")
	}

	stdout, err := runPythonSnippet(ctx, python, pullSnippet, modelID)
	if err != nil {
		return PullResult{}, fmt.Errorf("pull model %q: %w", modelID, err)
	}

	var result snippetResult
	if err := json.Unmarshal(stdout, &result); err != nil {
		return PullResult{}, fmt.Errorf("pull model %q: decode result: %w", modelID, err)
	}
	if result.Error != "" {
		if result.Error == ErrHuggingFaceHubUnavailable.Error() {
			return PullResult{}, fmt.Errorf("pull model %q: %w", modelID, ErrHuggingFaceHubUnavailable)
		}
		return PullResult{}, fmt.Errorf("pull model %q: %s", modelID, result.Error)
	}

	id := result.ID
	if id == "" {
		id = modelID
	}
	return PullResult{ID: id, SizeBytes: result.SizeBytes}, nil
}

// Delete removes modelID's weights from the local Hugging Face cache (see
// LocalCacheDir) using huggingface_hub's own eviction API, so shared
// content-addressed blobs belonging to other cached models are not
// corrupted. See Pull's doc comment for why this can't be a
// dependency-free filesystem operation the way ScanLocal is.
//
// modelID is passed to the subprocess as its own argv element, never
// interpolated into the Python source. Returns a wrapped
// ErrLocalModelNotFound if modelID has no cached revisions, and a wrapped
// ErrHuggingFaceHubUnavailable if huggingface_hub can't be imported.
func Delete(ctx context.Context, python PythonInterpreter, modelID string) (DeleteResult, error) {
	if modelID == "" {
		return DeleteResult{}, fmt.Errorf("delete model: model id must not be empty")
	}

	stdout, err := runPythonSnippet(ctx, python, deleteSnippet, modelID)
	if err != nil {
		return DeleteResult{}, fmt.Errorf("delete model %q: %w", modelID, err)
	}

	var result snippetResult
	if err := json.Unmarshal(stdout, &result); err != nil {
		return DeleteResult{}, fmt.Errorf("delete model %q: decode result: %w", modelID, err)
	}
	if result.Error != "" {
		switch result.Error {
		case ErrHuggingFaceHubUnavailable.Error():
			return DeleteResult{}, fmt.Errorf("delete model %q: %w", modelID, ErrHuggingFaceHubUnavailable)
		case "no_cached_model":
			return DeleteResult{}, fmt.Errorf("delete model %q: %w", modelID, ErrLocalModelNotFound)
		default:
			return DeleteResult{}, fmt.Errorf("delete model %q: %s", modelID, result.Error)
		}
	}

	id := result.ID
	if id == "" {
		id = modelID
	}
	return DeleteResult{ID: id, FreedBytes: result.FreedBytes}, nil
}
