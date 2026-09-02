package httpclient

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDoJSONSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer srv.Close()

	c := New(srv.URL, 5*time.Second)

	var out map[string]string
	err := c.DoJSON(context.Background(), "POST", "/foo", map[string]string{"a": "b"}, &out)
	if err != nil {
		t.Fatalf("DoJSON() error = %v", err)
	}
	if out["status"] != "ok" {
		t.Errorf("out[status] = %q, want ok", out["status"])
	}
}

func TestDoJSONNonSuccessStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("boom"))
	}))
	defer srv.Close()

	c := New(srv.URL, 5*time.Second)

	err := c.DoJSON(context.Background(), "GET", "/foo", nil, nil)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}

	var statusErr *StatusError
	if !isStatusError(err, &statusErr) {
		t.Fatalf("expected *StatusError, got %T: %v", err, err)
	}
	if statusErr.StatusCode != 500 {
		t.Errorf("StatusCode = %d, want 500", statusErr.StatusCode)
	}
}

func TestDoJSONRespectsContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := New(srv.URL, 5*time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := c.DoJSON(ctx, "GET", "/foo", nil, nil)
	if err == nil {
		t.Fatal("expected error for canceled context")
	}
}

func isStatusError(err error, target **StatusError) bool {
	se, ok := err.(*StatusError)
	if ok {
		*target = se
	}
	return ok
}
