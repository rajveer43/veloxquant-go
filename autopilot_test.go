package veloxquant

import (
	"context"
	"errors"
	"testing"
)

func TestAutoPilotSelectsModelForTask(t *testing.T) {
	c, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	session, err := c.AutoPilot(context.Background(), AutoPilotConfig{
		Task: "coding",
	})
	if err != nil {
		t.Fatalf("AutoPilot() error = %v", err)
	}

	plan := session.Plan()
	if plan.SelectedModel == "" {
		t.Error("expected a selected model")
	}
	if plan.ContextLength <= 0 {
		t.Error("expected positive context length")
	}
	if plan.Reason == "" {
		t.Error("expected non-empty reason")
	}
}

func TestAutoPilotExplicitModel(t *testing.T) {
	c, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	all := c.Models.List()
	if len(all) == 0 {
		t.Fatal("expected non-empty model registry")
	}
	want := all[0].Name

	session, err := c.AutoPilot(context.Background(), AutoPilotConfig{
		Model: want,
	})
	if err != nil {
		t.Fatalf("AutoPilot() error = %v", err)
	}

	if session.Plan().SelectedModel != want {
		t.Errorf("SelectedModel = %q, want %q", session.Plan().SelectedModel, want)
	}
}

func TestAutoPilotUnknownModelReturnsError(t *testing.T) {
	c, err := NewClient()
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	_, err = c.AutoPilot(context.Background(), AutoPilotConfig{
		Model: "does-not-exist",
	})
	if !errors.Is(err, ErrModelNotFound) {
		t.Errorf("expected ErrModelNotFound, got %v", err)
	}
}
