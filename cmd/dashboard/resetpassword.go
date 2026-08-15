package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/projecthelena/warden/internal/config"
	"github.com/projecthelena/warden/internal/db"
)

// runResetPassword implements `warden reset-password <username>`, the recovery path for a
// total lockout (no admin left, or nobody remembers any password). It talks to the same
// database the server uses and never starts the HTTP server, so it works offline.
func runResetPassword(args []string) int {
	if len(args) < 1 || strings.TrimSpace(args[0]) == "" {
		fmt.Fprintln(os.Stderr, "usage: warden reset-password <username>")
		return 2
	}
	username := args[0]

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		return 1
	}
	store, err := db.NewStore(db.DBConfig{Type: cfg.DBType, Path: cfg.DBPath, URL: cfg.DBURL})
	if err != nil {
		fmt.Fprintf(os.Stderr, "open database: %v\n", err)
		return 1
	}
	defer func() { _ = store.Close() }()

	password, err := promptNewPassword(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	if err := store.ResetPasswordByUsername(username, password); err != nil {
		if errors.Is(err, db.ErrUserNotFound) {
			fmt.Fprintf(os.Stderr, "no user named %q\n", username)
			return 1
		}
		fmt.Fprintf(os.Stderr, "reset failed: %v\n", err)
		return 1
	}

	fmt.Printf("password reset for %q; existing sessions were revoked\n", username)
	return 0
}

// promptNewPassword reads the new password. On a terminal it reads twice without echo and
// checks they match; when stdin is piped (scripts, tests) it reads a single line, so
// `echo 'newpass' | warden reset-password admin` works too.
func promptNewPassword(in *os.File) (string, error) {
	if term.IsTerminal(int(in.Fd())) {
		fmt.Print("New password: ")
		first, err := term.ReadPassword(int(in.Fd()))
		fmt.Println()
		if err != nil {
			return "", fmt.Errorf("read password: %w", err)
		}
		fmt.Print("Confirm password: ")
		second, err := term.ReadPassword(int(in.Fd()))
		fmt.Println()
		if err != nil {
			return "", fmt.Errorf("read password: %w", err)
		}
		if string(first) != string(second) {
			return "", errors.New("passwords do not match")
		}
		return string(first), nil
	}

	line, err := bufio.NewReader(in).ReadString('\n')
	line = strings.TrimRight(line, "\r\n")
	if err != nil && line == "" {
		return "", errors.New("no password provided on stdin")
	}
	return line, nil
}
