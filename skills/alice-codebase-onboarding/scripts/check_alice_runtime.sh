#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
default_repo="$(cd "$script_dir/../../.." && pwd -P)"
repo="${ALICE_REPO:-$default_repo}"
show_journal_lines=0
service_name="${ALICE_SERVICE_NAME:-alice-codex-connector.service}"

usage() {
  cat <<'USAGE'
Usage:
  check_alice_runtime.sh [--repo PATH] [--service NAME] [--journal N]

Options:
  --repo PATH      Target alice repository path
  --service NAME   User systemd service name (default: alice-codex-connector.service)
  --journal N      Print last N lines of user service journal (0 disables, default: 0)
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo)
      repo="${2:-}"
      shift 2
      ;;
    --service)
      service_name="${2:-}"
      shift 2
      ;;
    --journal)
      show_journal_lines="${2:-0}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

print_kv() {
  printf "%-26s %s\n" "$1" "$2"
}

echo "=== alice runtime quick check ==="
print_kv "timestamp" "$(date -Iseconds)"
print_kv "repo_path" "$repo"

if [[ ! -d "$repo" ]]; then
  print_kv "repo_exists" "no"
  exit 1
fi
print_kv "repo_exists" "yes"

if command -v git >/dev/null 2>&1; then
  if git -C "$repo" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    print_kv "git_branch" "$(git -C "$repo" rev-parse --abbrev-ref HEAD 2>/dev/null || echo unknown)"
    print_kv "git_commit" "$(git -C "$repo" rev-parse --short HEAD 2>/dev/null || echo unknown)"
    print_kv "git_dirty" "$(if [[ -n "$(git -C "$repo" status --porcelain 2>/dev/null)" ]]; then echo yes; else echo no; fi)"
  fi
fi

print_kv "go_version" "$(go version 2>/dev/null || echo missing)"
print_kv "codex_path" "$(command -v codex 2>/dev/null || echo missing)"

config_path="$repo/config.yaml"
if [[ -f "$config_path" ]]; then
  print_kv "config_yaml" "present"
  workspace_dir="$(awk -F': *' '$1=="workspace_dir"{print $2}' "$config_path" | tr -d '"' | head -n1 | xargs || true)"
  memory_dir="$(awk -F': *' '$1=="memory_dir"{print $2}' "$config_path" | tr -d '"' | head -n1 | xargs || true)"
  codex_command="$(awk -F': *' '$1=="codex_command"{print $2}' "$config_path" | tr -d '"' | head -n1 | xargs || true)"
  print_kv "cfg_workspace_dir" "${workspace_dir:-<unset>}"
  print_kv "cfg_memory_dir" "${memory_dir:-<unset>}"
  print_kv "cfg_codex_command" "${codex_command:-<unset>}"
else
  print_kv "config_yaml" "missing"
fi

binary_path="$repo/bin/alice-connector"
if [[ -x "$binary_path" ]]; then
  print_kv "binary" "present ($binary_path)"
else
  print_kv "binary" "missing ($binary_path)"
fi

if command -v systemctl >/dev/null 2>&1; then
  if systemctl --user list-unit-files "$service_name" >/dev/null 2>&1; then
    print_kv "user_service_file" "present"
    print_kv "user_service_state" "$(systemctl --user is-active "$service_name" 2>/dev/null || true)"
    print_kv "user_service_enabled" "$(systemctl --user is-enabled "$service_name" 2>/dev/null || true)"
  else
    print_kv "user_service_file" "not-found"
  fi
else
  print_kv "systemctl" "missing"
fi

if [[ "$show_journal_lines" =~ ^[0-9]+$ ]] && [[ "$show_journal_lines" -gt 0 ]]; then
  echo
  echo "=== user service journal (last $show_journal_lines lines) ==="
  journalctl --user-unit "$service_name" -n "$show_journal_lines" --no-pager 2>&1 || \
    journalctl --user -u "$service_name" -n "$show_journal_lines" --no-pager 2>&1 || true
fi
