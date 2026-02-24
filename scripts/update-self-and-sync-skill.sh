#!/usr/bin/env bash
set -euo pipefail

repo="/home/codexbot/alice"
service_name="alice-codex-connector.service"
skip_pull=0
skip_restart=0
sync_state_file="/home/codexbot/.codex/skills/alice-codebase-onboarding/references/sync-state.md"

usage() {
  cat <<'EOF'
Usage:
  update-self-and-sync-skill.sh [--repo PATH] [--service NAME] [--sync-state-file PATH] [--skip-pull] [--skip-restart]

Options:
  --repo PATH             Target alice repository path (default: /home/codexbot/alice)
  --service NAME          User systemd service name (default: alice-codex-connector.service)
  --sync-state-file PATH  Sync snapshot markdown path
  --skip-pull             Skip git pull (still build/restart/sync)
  --skip-restart          Skip systemd restart (still pull/build/sync)
EOF
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
    --sync-state-file)
      sync_state_file="${2:-}"
      shift 2
      ;;
    --skip-pull)
      skip_pull=1
      shift
      ;;
    --skip-restart)
      skip_restart=1
      shift
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

if [[ ! -d "$repo" ]]; then
  echo "repo not found: $repo" >&2
  exit 1
fi
if ! git -C "$repo" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "not a git repo: $repo" >&2
  exit 1
fi

before_commit="$(git -C "$repo" rev-parse --short HEAD)"
branch="$(git -C "$repo" rev-parse --abbrev-ref HEAD)"
pull_result="skipped"

if [[ "$skip_pull" -eq 0 ]]; then
  pull_output="$(git -C "$repo" pull --ff-only 2>&1)"
  pull_result="$pull_output"
fi

after_commit="$(git -C "$repo" rev-parse --short HEAD)"

(
  cd "$repo"
  go build -o bin/alice-connector ./cmd/connector
  go build -o bin/alice-mcp-server ./cmd/alice-mcp-server
)

restart_result="skipped"
if [[ "$skip_restart" -eq 0 ]]; then
  if command -v systemctl >/dev/null 2>&1; then
    export XDG_RUNTIME_DIR="/run/user/$(id -u)"
    export DBUS_SESSION_BUS_ADDRESS="unix:path=$XDG_RUNTIME_DIR/bus"
    systemctl --user restart --no-block "$service_name"
    sleep 1
    restart_result="$(systemctl --user is-active "$service_name" 2>/dev/null || true)"
  else
    restart_result="systemctl-missing"
  fi
fi

service_active="unknown"
service_enabled="unknown"
if command -v systemctl >/dev/null 2>&1; then
  service_active="$(systemctl --user is-active "$service_name" 2>/dev/null || echo unknown)"
  service_enabled="$(systemctl --user is-enabled "$service_name" 2>/dev/null || echo unknown)"
fi

last_commit_subject="$(git -C "$repo" log -1 --pretty=%s | tr -d '\r')"
updated_at="$(date -Iseconds)"

mkdir -p "$(dirname "$sync_state_file")"
cat >"$sync_state_file" <<EOF
# Skill Sync State

- updated_at: $updated_at
- repo_path: $repo
- branch: $branch
- before_commit: $before_commit
- after_commit: $after_commit
- last_commit_subject: $last_commit_subject
- service_name: $service_name
- service_active: $service_active
- service_enabled: $service_enabled
- skip_pull: $skip_pull
- skip_restart: $skip_restart

## Pull Result

\`\`\`
$pull_result
\`\`\`

## Restart Result

\`\`\`
$restart_result
\`\`\`
EOF

echo "=== update-self-and-sync-skill done ==="
echo "repo: $repo"
echo "branch: $branch"
echo "before_commit: $before_commit"
echo "after_commit: $after_commit"
echo "service_active: $service_active"
echo "service_enabled: $service_enabled"
echo "sync_state_file: $sync_state_file"
