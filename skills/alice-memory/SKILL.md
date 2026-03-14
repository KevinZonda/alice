---
name: alice-memory
description: 通过 Alice 本地 runtime HTTP API 查看或更新当前会话记忆。适用于查看当前会话记忆上下文、改写长期记忆、追加当日摘要，或确认会话正在使用的记忆文件。
---

# Alice 记忆

优先使用 `scripts/alice-memory.sh`，不要手工改 `.memory` 目录路径。脚本会读取 Alice 注入的当前会话上下文和鉴权环境变量。

## 常用命令

- 查看当前记忆上下文：
  `scripts/alice-memory.sh context`
- 覆盖当前会话长期记忆：
  `scripts/alice-memory.sh write-session '偏好中文回复；关注稳定性。'`
- 覆盖全局长期记忆：
  `scripts/alice-memory.sh write-global '所有会话默认先给结论。'`
- 为当前会话追加当日摘要：
  `scripts/alice-memory.sh daily-summary '今天确认了新的部署窗口和风险项。'`

## 工作流

1. 需要先搞清楚当前会话绑定了哪些记忆文件时，先执行 `context`。
2. 会话特有偏好或约束优先写 `write-session`。
3. 只有跨会话都稳定生效的规则才写入 `write-global`。
4. 有时效性的事项写入 `daily-summary`，保留在按日期日志中。

## 回复模式

- 明确说明你是“查看 / 覆盖 / 追加”了哪类记忆。
- 需要时带上 API 返回的实际文件路径。
- 除非用户明确要求，不要整文件回显，只做简要摘要。
