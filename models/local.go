package models

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LocalModel describes a model found in the local model cache directory.
type LocalModel struct {
	Name         string
	Path         string
	SizeBytes    uint64
	LastModified time.Time
}

// LocalCacheDir returns the platform-appropriate local model cache
// directory: $HF_HOME/hub if HF_HOME is set, otherwise
// ~/.cache/huggingface/hub. It returns "" if the home directory can't be
// determined.
func LocalCacheDir() string {
	if hfHome := os.Getenv("HF_HOME"); hfHome != "" {
		return filepath.Join(hfHome, "hub")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cache", "huggingface", "hub")
}

// ScanLocal scans the local model cache directory (see LocalCacheDir) and
// returns the models found on disk. It returns an empty slice, not an
// error, if the cache directory doesn't exist or can't be read.
func ScanLocal(ctx context.Context) ([]LocalModel, error) {
	return scanLocalDir(ctx, LocalCacheDir())
}

// scanLocalDir is the implementation behind ScanLocal, taking an explicit
// directory so tests can point it at a temporary cache without touching
// HF_HOME or the real user cache directory.
func scanLocalDir(ctx context.Context, dir string) ([]LocalModel, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if dir == "" {
		return []LocalModel{}, nil
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return []LocalModel{}, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []LocalModel{}, nil
	}
	var results []LocalModel
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !entry.IsDir() {
			continue
		}
		entryName := entry.Name()
		if !strings.HasPrefix(entryName, "models--") {
			continue
		}
		modelDir := filepath.Join(dir, entryName)
		var totalSize uint64
		var latestMod time.Time
		_ = filepath.WalkDir(modelDir, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			if !d.IsDir() {
				totalSize += uint64(info.Size())
			}
			if info.ModTime().After(latestMod) {
				latestMod = info.ModTime()
			}
			return nil
		})
		cleanName := strings.TrimPrefix(entryName, "models--")
		cleanName = strings.ReplaceAll(cleanName, "--", "/")
		results = append(results, LocalModel{
			Name:         cleanName,
			Path:         modelDir,
			SizeBytes:    totalSize,
			LastModified: latestMod,
		})
	}
	return results, nil
}
