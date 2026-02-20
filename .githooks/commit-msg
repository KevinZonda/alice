#!/usr/bin/env bash
set -euo pipefail

msg_file="${1:-}"
if [[ -z "${msg_file}" || ! -f "${msg_file}" ]]; then
  echo "[commit-msg] missing commit message file"
  exit 1
fi

first_line="$(head -n 1 "${msg_file}" | tr -d '\r')"

# Enforce Conventional Commits:
# type(scope)!: subject
# type: subject
pattern='^(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert)(\([a-z0-9._/-]+\))?(!)?: .+$'
if [[ ! "${first_line}" =~ ${pattern} ]]; then
  cat <<'EOF'
[commit-msg] invalid commit message.
Expected Conventional Commits format:
  type(scope): subject
  type: subject

Examples:
  feat(connector): support codex resume thread
  fix: keep proxy env for codex exec
EOF
  echo "[commit-msg] got: ${first_line}"
  exit 1
fi

