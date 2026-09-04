package models

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type LocalModel struct {
	Name         string
	Path         string
	SizeBytes    uint64
	LastModified time.Time
}

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
func ScanLocal(ctx context.Context, customDir ...string) ([]LocalModel, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	dir := LocalCacheDir()
	if len(customDir) > 0 && customDir[0] != "" {
		dir = customDir[0]
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
