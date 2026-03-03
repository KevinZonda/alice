---
name: alice-code-army
description: Operate Alice's built-in `code_army` workflow through Alice Feishu automation tools. Use when the user asks to start, continue, inspect, pause, resume, delete, or test a `code army` / `code_army` workflow in the current Feishu conversation, especially for one-off coding iterations or recurring workflow runs.
---

# Alice Code Army

Run `code_army` via Alice automation tools instead of re-reading the repository. Keep actions scoped to the current conversation and rely on Alice to route replies automatically.

## Defaults

- Use `mcp__alice-feishu__automation_task_create` to start runs.
- Use `mcp__alice-feishu__automation_task_list`, `mcp__alice-feishu__automation_task_get`, `mcp__alice-feishu__automation_task_update`, and `mcp__alice-feishu__automation_task_delete` to manage tasks.
- Use `mcp__alice-feishu__code_army_status_get` to inspect workflow state in the current conversation.
- Set `action_type` to `run_workflow` and `workflow` to `code_army`.
- For a one-off test run, create an interval task with `every_seconds: 60` and `max_runs: 1`.
- `code_army` does not have an immediate-run API. The first execution happens at `next_run_at`, so tell the user the exact scheduled time.
- Reuse an explicit `state_key` when the user may run multiple workflows in the same conversation.
- Reusing the same `state_key` continues the existing state. The stored objective is retained, so later prompts should stay aligned with the original goal instead of trying to redefine it.
- Only set `model` or `profile` when the user asks for them or the surrounding workflow already depends on them.

## Start A One-Off Run

1. Turn the user request into a concrete workflow objective.
2. Pick a short `state_key` if the run should be inspectable or resumable later.
3. Create a one-off task with `schedule_type: "interval"`, `every_seconds: 60`, `action_type: "run_workflow"`, `workflow: "code_army"`, and `max_runs: 1`.
4. After creation, report `task.id`, `next_run`, and the `state_key` you used.

Example:

```text
mcp__alice-feishu__automation_task_create({
  title: "code_army: rust calculator",
  schedule_type: "interval",
  every_seconds: 60,
  action_type: "run_workflow",
  workflow: "code_army",
  state_key: "rust-cli-calculator",
  prompt: "制作一个使用 Rust 编写的终端计算器，支持加减乘除即可。按 code_army 工作流推进一轮，并在回复中给出当前阶段进展。",
  max_runs: 1
})
```

## Continue Or Iterate

Use the same `state_key` to continue an existing workflow state. The simplest pattern is to create another one-off task for the next round:

```text
mcp__alice-feishu__automation_task_create({
  title: "code_army: continue rust calculator",
  schedule_type: "interval",
  every_seconds: 60,
  action_type: "run_workflow",
  workflow: "code_army",
  state_key: "rust-cli-calculator",
  prompt: "继续推进 rust-cli-calculator 这一条 code_army 工作流。",
  max_runs: 1
})
```

If the user wants an always-on loop instead of manual nudges, create or update a recurring task with either a larger interval or a cron schedule.

## Inspect State

Call `mcp__alice-feishu__code_army_status_get` with or without `state_key`.

- Without `state_key`: list all `code_army` states in the current conversation.
- With `state_key`: load the exact workflow snapshot.
- Read these fields first: `phase`, `iteration`, `last_decision`, `updated_at`, `history`.

Phase semantics:

- `manager`: planning the current iteration
- `worker`: producing the implementation plan/output
- `reviewer`: reviewing the worker result
- `gate`: deciding whether to advance to the next iteration or send the workflow back to `worker`

Interpretation:

- `last_decision: "pass"` means the next gate will advance to the next iteration.
- `last_decision: "fail"` means the next gate will send the workflow back for rework.
- `history` is the quickest way to summarize what changed since the previous run.

## Manage Tasks

- List tasks in the current scope before editing or deleting when the task id is not already known.
- Pause or resume with `mcp__alice-feishu__automation_task_update`.
- Change cadence with `mcp__alice-feishu__automation_task_update`.
- Remove obsolete tasks with `mcp__alice-feishu__automation_task_delete`.

## Cron Note

Alice automation currently validates plain 5-field cron expressions. Do not prepend `CRON_TZ=...` inside `cron_expr`. When the user wants a Shanghai-time schedule, compute the UTC-equivalent 5-field cron value, state the intended `Asia/Shanghai` time explicitly in the reply, and note the conversion.

## Reply Pattern

When you operate this workflow, report:

- whether you created, updated, listed, inspected, or deleted a task
- the `task.id` and `state_key` when relevant
- the exact `next_run_at` for newly created or rescheduled tasks
- whether the run is one-off or recurring
- any limitation that affects user expectations, especially the lack of immediate execution
