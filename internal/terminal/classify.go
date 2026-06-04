package terminal

import "strings"

var readOnlyCommands = map[string]struct{}{
	"pwd":  {},
	"ls":   {},
	"cat":  {},
	"head": {},
	"tail": {},
	"less": {},
	"rg":   {},
	"grep": {},
}

var readOnlyGitSubcommands = map[string]struct{}{
	"status":    {},
	"diff":      {},
	"log":       {},
	"show":      {},
	"branch":    {},
	"rev-parse": {},
}

// ClassifyCommand must stay in lockstep with frontend/src/lib/commandSafety.js.
// Block-derived commands are untrusted data; only strict read-only commands may skip confirmation.
func ClassifyCommand(command string) (readOnly bool, reason string) {
	cmd := strings.TrimSpace(command)
	if cmd == "" {
		return false, "empty command"
	}
	if containsShellControl(cmd) {
		return false, "contains shell control characters"
	}

	fields := strings.Fields(cmd)
	if len(fields) == 0 {
		return false, "empty command"
	}

	bin := fields[0]
	if bin == "git" {
		if len(fields) < 2 {
			return false, "git subcommand is required"
		}
		if _, ok := readOnlyGitSubcommands[fields[1]]; !ok {
			return false, "git subcommand is not read-only"
		}
		return true, "read-only git command"
	}
	if _, ok := readOnlyCommands[bin]; ok {
		return true, "read-only command"
	}
	return false, "command is not allowlisted"
}

func containsShellControl(command string) bool {
	return strings.ContainsAny(command, ";\n\r|&`$><()")
}
