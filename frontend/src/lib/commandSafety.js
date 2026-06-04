const READ_ONLY_COMMANDS = new Set(['pwd', 'ls', 'cat', 'head', 'tail', 'less', 'rg', 'grep'])
const READ_ONLY_GIT_SUBCOMMANDS = new Set(['status', 'diff', 'log', 'show', 'branch', 'rev-parse'])
const SHELL_CONTROL_RE = /[;\n\r|&`$><()]/

/**
 * Must stay in lockstep with internal/terminal/classify.go.
 * Block-derived commands are untrusted data; only strict read-only commands may skip confirmation.
 *
 * @param {string} command
 * @returns {{ readOnly: boolean, reason: string }}
 */
export function classifyCommand(command) {
  const cmd = String(command || '').trim()
  if (!cmd) return { readOnly: false, reason: 'empty command' }
  if (SHELL_CONTROL_RE.test(cmd)) return { readOnly: false, reason: 'contains shell control characters' }

  const fields = cmd.split(/\s+/).filter(Boolean)
  if (!fields.length) return { readOnly: false, reason: 'empty command' }

  const bin = fields[0]
  if (bin === 'git') {
    if (fields.length < 2) return { readOnly: false, reason: 'git subcommand is required' }
    if (!READ_ONLY_GIT_SUBCOMMANDS.has(fields[1])) {
      return { readOnly: false, reason: 'git subcommand is not read-only' }
    }
    return { readOnly: true, reason: 'read-only git command' }
  }

  if (READ_ONLY_COMMANDS.has(bin)) return { readOnly: true, reason: 'read-only command' }
  return { readOnly: false, reason: 'command is not allowlisted' }
}
