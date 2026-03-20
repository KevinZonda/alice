---
name: alice-scheduler
description: 通过 Alice 本地 runtime HTTP API 管理当前会话的自动化任务。适用于创建、列出、查看、补丁更新、暂停、恢复、删除任务，以及处理 `send_text` / `run_llm` 任务。
---

# Alice 调度器

使用 `scripts/alice-scheduler.sh` 管理当前会话自动化任务。脚本会自动使用本地 runtime HTTP API 与当前会话上下文。

## 常用命令

- 列出当前作用域任务：
  `scripts/alice-scheduler.sh list`
- 用 JSON 创建任务：
  `scripts/alice-scheduler.sh create <<'JSON'`
  `{ "title": "daily sync", "schedule": { "type": "cron", "cron_expr": "0 1 * * *" }, "action": { "type": "run_llm", "prompt": "总结今天的进展" } }`
  `JSON`
- 查看单个任务：
  `scripts/alice-scheduler.sh get task_xxx`
- 用 merge patch 更新任务：
  `scripts/alice-scheduler.sh patch task_xxx '{"status":"paused"}'`
- 删除任务：
  `scripts/alice-scheduler.sh delete task_xxx`
## 任务结构

- `schedule.type`：`interval` 或 `cron`
- `schedule.every_seconds`：`interval` 必填，最小 `60`
- `schedule.cron_expr`：`cron` 必填
- `action.type`：`send_text`、`run_llm`
- `manage_mode`：`creator_only` 或 `scope_all`（`scope_all` 仅群聊有意义）

## 工作流

1. 不知道任务 ID 时，先 `list` 再改删。
2. 更新任务优先用小范围 `patch`，不要整对象重写。
3. 一次性执行推荐：`interval + every_seconds: 60 + max_runs: 1`。

## 回复模式

- 明确说明执行了什么操作，以及对应的 `task.id`。
- 新建或重排任务时，给出精确 `next_run_at`。
- 说明这是一次性任务还是周期任务。
