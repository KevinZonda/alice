# 飞书 Task v2 API 对照表

## 系统边界

- `Alice任务`：连接器本地自动化任务。
- `飞书任务`：飞书官方 OpenAPI Task 资源。
- 本 skill 仅用于 `飞书任务`。

## Token 规则

- `GET /open-apis/task/v2/tasks`（列表）在官方 SDK 元数据中要求 `user_access_token`。
- 其余多数 Task v2 端点支持 `tenant token` 与 `user token`。
- 涉及“当前人的任务”时优先使用 `user_access_token`。

## CRUD 覆盖

### 任务 CRUD

- 创建任务：`POST /open-apis/task/v2/tasks`
- 获取任务：`GET /open-apis/task/v2/tasks/:task_guid`
- 列表任务：`GET /open-apis/task/v2/tasks`
- 更新任务：`PATCH /open-apis/task/v2/tasks/:task_guid`
- 删除任务：`DELETE /open-apis/task/v2/tasks/:task_guid`

### 任务成员 / 分派

- 添加成员：`POST /open-apis/task/v2/tasks/:task_guid/add_members`
- 移除成员：`POST /open-apis/task/v2/tasks/:task_guid/remove_members`

### 截止时间

- 更新截止时间：`PATCH /open-apis/task/v2/tasks/:task_guid`，字段为：
  - `task.due.timestamp`（毫秒）
  - `task.due.is_all_day`（布尔）
  - `update_fields: ["due"]`

### 任务清单 CRUD

- 创建清单：`POST /open-apis/task/v2/tasklists`
- 获取清单：`GET /open-apis/task/v2/tasklists/:tasklist_guid`
- 列表清单：`GET /open-apis/task/v2/tasklists`
- 更新清单：`PATCH /open-apis/task/v2/tasklists/:tasklist_guid`
- 删除清单：`DELETE /open-apis/task/v2/tasklists/:tasklist_guid`

### 清单成员

- 添加清单成员：`POST /open-apis/task/v2/tasklists/:tasklist_guid/add_members`
- 移除清单成员：`POST /open-apis/task/v2/tasklists/:tasklist_guid/remove_members`

### 清单内任务查询

- 查询清单内任务：`GET /open-apis/task/v2/tasklists/:tasklist_guid/tasks`

## 用户诉求 -> API 映射

- 检查“你当前管理的任务”：
  - `GET /task/v2/tasklists`
  - `GET /task/v2/tasklists/:tasklist_guid/tasks`
- 查看“当前人的任务”：
  - `GET /task/v2/tasks?type=my_tasks`
- 发布新任务：
  - `POST /task/v2/tasks`
- assign 特定成员：
  - `POST /task/v2/tasks/:task_guid/add_members`
- 设置 deadline：
  - `PATCH /task/v2/tasks/:task_guid`（`update_fields=["due"]`）
- 创建和管理任务清单：
  - `POST/GET/PATCH/DELETE /task/v2/tasklists...`

## SDK 参考

- Node SDK：
  - 包：`@larksuiteoapi/node-sdk`
  - 命名空间示例：
    - `client.task.v2.task.create(...)`
    - `client.task.v2.task.addMembers(...)`
    - `client.task.v2.tasklist.create(...)`
- Go SDK：
  - 模块：`github.com/larksuite/oapi-sdk-go/v3`
  - 命名空间示例：
    - `client.Task.V2.Task.Create(...)`
    - `client.Task.V2.Task.AddMembers(...)`
    - `client.Task.V2.Tasklist.Create(...)`
