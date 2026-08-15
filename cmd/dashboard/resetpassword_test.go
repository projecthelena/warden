package main

import (
	"os"
	"testing"
)

// promptNewPassword must read a piped (non-TTY) password as a single line, which is what
// makes the command scriptable and testable.
func TestPromptNewPassword_Stdin(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	go func() {
		_, _ = w.WriteString("Secret123!\n")
		_ = w.Close()
	}()

	got, err := promptNewPassword(r)
	if err != nil {
		t.Fatalf("promptNewPassword: %v", err)
	}
	if got != "Secret123!" {
		t.Errorf("got %q, want %q", got, "Secret123!")
	}
}

func TestPromptNewPassword_EmptyStdin(t *testing.T) {
	r, w, _ := os.Pipe()
	_ = w.Close() // nothing written

	if _, err := promptNewPassword(r); err == nil {
		t.Error("empty stdin should error, not return an empty password")
	}
}
