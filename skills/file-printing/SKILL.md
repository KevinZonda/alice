---
name: file-printing
description: 'Print user-provided files through CUPS on this machine. Use when users ask to print an attached file or local file path, check or set the default printer, inspect print queue status, or troubleshoot failed print jobs (for example: "打印这个文件", "用HL5590打印", "查看打印任务状态").'
---

# File Printing

Follow this workflow to print files reliably and report actionable status.

## Workflow

1. Resolve file input.
- Prefer attachment path from user message context.
- Accept absolute local path when provided directly.
- Verify readability with `test -r <file_path>` before submitting.

2. Resolve target printer.
- Prefer explicit printer from user instruction.
- Otherwise use current user default from `lpoptions` (`Default <printer>`).
- If missing and user does not object, set default to `HL5590` via `lpoptions -d HL5590`.

3. Submit print job.
- Run `scripts/print_file.sh <file_path>` for default behavior.
- Add options only when requested: `--printer`, `--copies`, `--sides`, `--media`, `--option`.
- Return `JOB_ID` and `PRINTER` to user after successful submission.

4. Confirm queue status.
- Check active jobs with `lpstat -W not-completed -o` (or `lpstat -o`).
- Report whether the job is queued/printing and include the relevant line.

5. Handle errors explicitly.
- If output contains `Forbidden`, explain this is a permission restriction from CUPS policy and ask whether to proceed with user-level fallback/default.
- If file format fails, run `file --mime-type <file_path>` and suggest converting to PDF before reprint.
- If printer is unavailable, run `lpstat -p <printer>` and report state/reason.

## Commands

- Set default printer (current user): `lpoptions -d HL5590`
- Show default printer (current user): `lpoptions | sed -n '1p'`
- Show configured printers: `lpstat -p`
- Show device mapping: `lpstat -v`
- Show queue: `lpstat -W not-completed -o`

## Script

- Main script: `scripts/print_file.sh`
- Purpose: Validate input, submit print job via `lp`, and emit parse-friendly output (`PRINTER`, `JOB_ID`, `RAW`).
- Safe check before real print: `scripts/print_file.sh --dry-run <file_path>`

## Example

User: "把这个PDF打印两份，双面长边。"

Run:
`scripts/print_file.sh /abs/path/file.pdf --copies 2 --sides two-sided-long-edge`

Reply with:
- printer name
- job id
- queue status line
