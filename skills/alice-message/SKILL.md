---
name: alice-message
description: 通过 Alice 本地 runtime HTTP API 向当前会话发送文本、图片或文件。适用于在当前聊天里回复、跟进、发送本地文件/图片，或复用已有飞书 `image_key` / `file_key`。
---

# Alice 消息发送

使用 `scripts/alice-message.sh` 把内容发送回当前 Alice 会话。脚本会自动读取当前会话上下文，并由 Alice 自动路由普通回复/话题回复。

## 常用命令

- 发送文本：
  `scripts/alice-message.sh text '已完成，见附件。'`
- 发送本地图片（先上传后发送）：
  `scripts/alice-message.sh image --path /abs/path/image.png --caption '最新截图'`
- 发送已有飞书图片：
  `scripts/alice-message.sh image --image-key img_v3_xxx`
- 发送本地文件（先上传后发送）：
  `scripts/alice-message.sh file --path /abs/path/report.pdf --file-name report.pdf --caption '请查收'`
- 发送已有飞书文件：
  `scripts/alice-message.sh file --file-key file_v3_xxx`

## 工作流

1. 纯文本跟进优先用 `text`。
2. 已有本地绝对路径时用 `image --path` 或 `file --path`。
3. 资源已上传过时优先复用 `--image-key` / `--file-key`。
4. 不要要求用户提供 `receive_id_type`、`receive_id`、`source_message_id`；由 Alice 根据当前会话自动解析。

## 回复模式

- 说明发送了哪种类型（文本 / 图片 / 文件）。
- 需要时给出使用的本地路径或飞书 key。
- 如果因路径不在当前会话资源根目录导致上传失败，要明确说明并停止继续重试。
