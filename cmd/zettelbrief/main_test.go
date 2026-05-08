package main

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillCreateValidation(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{name: "missing scope", args: []string{"skill", "create", "--default"}, wantErr: "exactly one scope is required"},
		{name: "conflicting scopes", args: []string{"skill", "create", "--user", "--project", "--default"}, wantErr: "exactly one scope is required"},
		{name: "missing target", args: []string{"skill", "create", "--project"}, wantErr: "at least one agent target is required"},
		{name: "unsupported general target", args: []string{"skill", "create", "--user", "--general"}, wantErr: "unknown flag: --general"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := executeRootCommand(tt.args...)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err=%v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestSkillCreateMultipleTargets(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	t.Setenv("HOME", home)
	out, _, err := executeRootCommand("skill", "create", "--user", "--default", "--claude")
	if err != nil {
		t.Fatalf("skill create failed: %v\n%s", err, out)
	}
	for _, path := range []string{
		filepath.Join(home, ".agents", "skills", "zettelbrief", "SKILL.md"),
		filepath.Join(home, ".claude", "skills", "zettelbrief", "SKILL.md"),
	} {
		if !strings.Contains(out, path) {
			t.Fatalf("output missing %s:\n%s", path, out)
		}
	}
}

func TestSkillCreateHelpIsDiscoverable(t *testing.T) {
	out, _, err := executeRootCommand("skill", "create", "--help")
	if err != nil {
		t.Fatalf("help failed: %v", err)
	}
	for _, flag := range []string{"--user", "--project", "--default", "--claude", "--force"} {
		if !strings.Contains(out, flag) {
			t.Fatalf("help missing %s:\n%s", flag, out)
		}
	}
	if strings.Contains(out, "--general") {
		t.Fatalf("help must not mention --general:\n%s", out)
	}
}

func executeRootCommand(args ...string) (string, string, error) {
	cmd := newRootCommand()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}
