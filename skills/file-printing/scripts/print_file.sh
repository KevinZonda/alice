#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage:
  print_file.sh [options] <file_path>

Options:
  --printer <name>              Target printer (default: user default, fallback HL5590)
  --copies <n>                  Number of copies (default: 1)
  --sides <mode>                one-sided | two-sided-long-edge | two-sided-short-edge
  --media <value>               Media setting, e.g. A4
  --option <key=value>          Extra lp -o option (repeatable)
  --dry-run                     Show resolved command without submitting a print job
  -h, --help                    Show this help message
EOF
}

require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "ERROR: required command not found: $cmd" >&2
    exit 1
  fi
}

PRINTER=""
COPIES="1"
SIDES=""
MEDIA=""
DRY_RUN=0
FILE_PATH=""
EXTRA_OPTIONS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --printer)
      [[ $# -ge 2 ]] || { echo "ERROR: --printer requires a value" >&2; exit 1; }
      PRINTER="$2"
      shift 2
      ;;
    --copies)
      [[ $# -ge 2 ]] || { echo "ERROR: --copies requires a value" >&2; exit 1; }
      COPIES="$2"
      shift 2
      ;;
    --sides)
      [[ $# -ge 2 ]] || { echo "ERROR: --sides requires a value" >&2; exit 1; }
      SIDES="$2"
      shift 2
      ;;
    --media)
      [[ $# -ge 2 ]] || { echo "ERROR: --media requires a value" >&2; exit 1; }
      MEDIA="$2"
      shift 2
      ;;
    --option)
      [[ $# -ge 2 ]] || { echo "ERROR: --option requires a value" >&2; exit 1; }
      EXTRA_OPTIONS+=("$2")
      shift 2
      ;;
    --dry-run)
      DRY_RUN=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    --*)
      echo "ERROR: unknown option: $1" >&2
      usage >&2
      exit 1
      ;;
    *)
      if [[ -n "$FILE_PATH" ]]; then
        echo "ERROR: only one file path is supported per invocation" >&2
        exit 1
      fi
      FILE_PATH="$1"
      shift
      ;;
  esac
done

if [[ -z "$FILE_PATH" ]]; then
  echo "ERROR: missing file path" >&2
  usage >&2
  exit 1
fi

if [[ ! -f "$FILE_PATH" ]]; then
  echo "ERROR: file does not exist: $FILE_PATH" >&2
  exit 1
fi

if [[ ! -r "$FILE_PATH" ]]; then
  echo "ERROR: file is not readable: $FILE_PATH" >&2
  exit 1
fi

if [[ ! "$COPIES" =~ ^[1-9][0-9]*$ ]]; then
  echo "ERROR: --copies must be a positive integer (got: $COPIES)" >&2
  exit 1
fi

if [[ -n "$SIDES" ]]; then
  case "$SIDES" in
    one-sided|two-sided-long-edge|two-sided-short-edge) ;;
    *)
      echo "ERROR: invalid --sides value: $SIDES" >&2
      exit 1
      ;;
  esac
fi

require_cmd lp
require_cmd lpoptions

if [[ -z "$PRINTER" ]]; then
  PRINTER="$(lpoptions 2>/dev/null | awk '/^Default /{print $2; exit}')"
fi
if [[ -z "$PRINTER" ]]; then
  PRINTER="HL5590"
fi

LP_CMD=(lp -d "$PRINTER" -n "$COPIES")
if [[ -n "$SIDES" ]]; then
  LP_CMD+=(-o "sides=$SIDES")
fi
if [[ -n "$MEDIA" ]]; then
  LP_CMD+=(-o "media=$MEDIA")
fi
for opt in "${EXTRA_OPTIONS[@]}"; do
  LP_CMD+=(-o "$opt")
done
LP_CMD+=("$FILE_PATH")

if [[ "$DRY_RUN" -eq 1 ]]; then
  printf 'DRY_RUN_COMMAND='
  printf '%q ' "${LP_CMD[@]}"
  printf '\n'
  exit 0
fi

OUTPUT="$("${LP_CMD[@]}" 2>&1)" || {
  echo "ERROR: failed to submit print job" >&2
  echo "DETAIL: $OUTPUT" >&2
  exit 1
}

JOB_ID="$(printf '%s\n' "$OUTPUT" | sed -n 's/^request id is \([^ ]*\).*/\1/p')"
echo "PRINTER=$PRINTER"
if [[ -n "$JOB_ID" ]]; then
  echo "JOB_ID=$JOB_ID"
else
  echo "JOB_ID=unknown"
fi
echo "RAW=$OUTPUT"

if command -v lpstat >/dev/null 2>&1; then
  QUEUE_LINE="$(lpstat -W not-completed -o 2>/dev/null | awk -v id="$JOB_ID" '$1 == id {print; exit}')"
  if [[ -n "$QUEUE_LINE" ]]; then
    echo "QUEUE=$QUEUE_LINE"
  fi
fi
