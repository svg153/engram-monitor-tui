package cli

import (
	"os"
	"testing"
)

func TestDefaultAddrEnv(t *testing.T) {
	orig := os.Getenv("ENGRAM_ADDR")
	defer os.Setenv("ENGRAM_ADDR", orig)

	os.Unsetenv("ENGRAM_ADDR")
	if got := defaultAddr(); got != "http://127.0.0.1:7437" {
		t.Errorf("defaultAddr() = %q, want %q", got, "http://127.0.0.1:7437")
	}

	os.Setenv("ENGRAM_ADDR", "http://custom:8080")
	if got := defaultAddr(); got != "http://custom:8080" {
		t.Errorf("defaultAddr() with ENGRAM_ADDR = %q, want %q", got, "http://custom:8080")
	}

	os.Setenv("ENGRAM_ADDR", "")
	if got := defaultAddr(); got != "http://127.0.0.1:7437" {
		t.Errorf("defaultAddr() with empty ENGRAM_ADDR = %q, want %q", got, "http://127.0.0.1:7437")
	}
}
