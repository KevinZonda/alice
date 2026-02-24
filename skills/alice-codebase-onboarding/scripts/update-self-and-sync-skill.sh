#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
default_repo="$(cd "$script_dir/../../.." && pwd -P)"
repo="${ALICE_REPO:-$default_repo}"
args=("$@")

for ((i=1; i<=$#; i++)); do
  if [[ "${!i}" == "--repo" ]]; then
    next=$((i + 1))
    if (( next <= $# )); then
      repo="${!next}"
    fi
    break
  fi
done

delegate="$repo/scripts/update-self-and-sync-skill.sh"
if [[ ! -x "$delegate" ]]; then
  echo "missing executable updater script: $delegate" >&2
  exit 1
fi

exec "$delegate" "${args[@]}"
