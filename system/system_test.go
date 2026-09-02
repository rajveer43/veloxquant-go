package system

import (
	"context"
	"testing"
	"time"
)

func TestNewDetectorInfoDoesNotError(t *testing.T) {
	d := NewDetector()

	info, err := d.Info(context.Background())
	if err != nil {
		t.Fatalf("Info() error = %v", err)
	}

	if info.Platform == "" {
		t.Error("expected non-empty Platform")
	}
	if info.Architecture == "" {
		t.Error("expected non-empty Architecture")
	}
	if info.RecommendedProfile == "" {
		t.Error("expected non-empty RecommendedProfile")
	}
}

func TestInfoRespectsCanceledContext(t *testing.T) {
	d := NewDetector()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := d.Info(ctx)
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
}

func TestInfoRespectsTimeout(t *testing.T) {
	d := NewDetector()

	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond)

	_, err := d.Info(ctx)
	if err == nil {
		t.Fatal("expected error for expired context")
	}
}

// mockDetector lets other packages' tests (and this one) verify behavior
// against known hardware profiles without depending on the real host.
type mockDetector struct {
	info Info
	err  error
}

func (m mockDetector) Info(ctx context.Context) (Info, error) {
	if m.err != nil {
		return Info{}, m.err
	}
	return m.info, nil
}

func TestRecommendProfileThresholds(t *testing.T) {
	tests := []struct {
		name        string
		totalMemory uint64
		want        string
	}{
		{"unknown memory", 0, "balanced"},
		{"4GB low memory", 4 * gib, "memory"},
		{"16GB balanced", 16 * gib, "balanced"},
		{"24GB balanced", 24 * gib, "balanced"},
		{"64GB maximum context", 64 * gib, "maximum-context"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := recommendProfile(Info{TotalMemory: tt.totalMemory})
			if got != tt.want {
				t.Errorf("recommendProfile(%d) = %s, want %s", tt.totalMemory, got, tt.want)
			}
		})
	}
}

const gib = 1024 * 1024 * 1024

func TestMockDetectorSatisfiesInterface(t *testing.T) {
	var _ Detector = mockDetector{}

	m := mockDetector{info: Info{Platform: "darwin", AppleSilicon: true, TotalMemory: 24 * gib}}
	info, err := m.Info(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !info.AppleSilicon {
		t.Error("expected AppleSilicon true from mock")
	}
}
