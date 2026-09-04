package models

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestScanLocal_EmptyDir(t *testing.T) {
	ctx := context.Background()

	models, err := scanLocalDir(ctx, "")
	if err != nil {
		t.Fatalf("expected nil error on empty directory, got %v", err)
	}
	if len(models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(models))
	}
}

func TestScanLocal_MissingDir(t *testing.T) {
	ctx := context.Background()
	missingDir := filepath.Join(t.TempDir(), "non-existent")

	models, err := scanLocalDir(ctx, missingDir)
	if err != nil {
		t.Fatalf("expected nil error on missing directory, got %v", err)
	}
	if len(models) != 0 {
		t.Fatalf("expected 0 models, got %d", len(models))
	}
}

func TestScanLocal_DiscoversModels(t *testing.T) {
	ctx := context.Background()
	cacheDir := t.TempDir()

	modelDir := filepath.Join(cacheDir, "models--testorg--testmodel")
	if err := os.MkdirAll(modelDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write dummy weights file (100 bytes)
	dummyData := make([]byte, 100)
	if err := os.WriteFile(filepath.Join(modelDir, "weights.safetensors"), dummyData, 0644); err != nil {
		t.Fatal(err)
	}

	models, err := scanLocalDir(ctx, cacheDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(models))
	}

	if models[0].Name != "testorg/testmodel" {
		t.Errorf("expected name 'testorg/testmodel', got %q", models[0].Name)
	}

	if models[0].SizeBytes != 100 {
		t.Errorf("expected size 100, got %d", models[0].SizeBytes)
	}
}
