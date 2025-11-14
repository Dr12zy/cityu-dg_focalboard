# AI AI 接口使用说明（简体中文）

本文档针对 `server/api/creat.go` 与 `server/api/modify.go` 中实现的 AI 接口，提供面向客户端开发者的简体中文说明，涵盖功能介绍、使用方式与示例。

---

## 1. 创建卡片接口（`creat.go`）

- **请求方法**：`POST`
- **URL**：`/api/v2/ai/cards/create`
- **认证方式**：`Authorization: Bearer <token>`
- **适用场景**：AI 或自动化系统在指定看板内生成新的任务卡片。

### 1.1 功能说明

1. 校验请求体中的 `boardId` 并确保调用者对该看板拥有 `PermissionManageBoardCards` 权限。
2. 自动将卡片绑定到目标看板并调用 `CreateCard` 完成写入。
3. 支持通过 `disable_notify=true` 禁用通知，用于批量导入或后台处理。
4. 全流程写入审计日志，便于追踪 AI 操作。

### 1.2 请求参数


| 位置  | 参数             | 是否必填 | 说明                         |
| ------- | ------------------ | ---------- | ------------------------------ |
| Query | `disable_notify` | 否       | `true/false`，默认 `false`   |
| Body  | `boardId`        | 是       | 目标看板 ID                  |
| Body  | `title`          | 否       | 卡片标题                     |
| Body  | `contentOrder`   | 否       | 内容块 ID 数组，默认空       |
| Body  | `properties`     | 否       | 属性键值对，例如状态、优先级 |

> 其余字段（如 `icon`、`isTemplate`）会按 `model.Card` 定义原样透传。

### 1.3 示例

```bash
curl -X POST http://localhost:8000/api/v2/ai/cards/create \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <TOKEN>" \
  -H "X-Requested-With: XMLHttpRequest" \
  -d '{
    "boardId": "board-id-123",
    "title": "AI 生成任务",
    "properties": {
      "priority": "High"
    }
  }'
```

**成功响应**：返回完整的卡片 JSON，包含 `id`、`boardId`、`properties` 等字段。
**常见错误**：缺少 `boardId`（400）、权限不足（403）、会话无效（401）。

---

## 2. 修改状态接口（`modify.go`）

- **请求方法**：`POST`
- **URL**：`/api/v2/ai/cards/modify`
- **认证方式**：`Authorization: Bearer <token>`
- **适用场景**：AI 根据任务进度自动更新卡片状态，不需要硬编码属性 ID。

### 2.1 功能说明

1. 根据 `cardId` 读取目标卡片并校验调用者是否拥有 `PermissionManageBoardCards` 权限。
2. 自动遍历所属看板的 `cardProperties`，找到名为 `"Status"` 的属性 ID。
3. 构造 `CardPatch`，仅更新状态属性，避免覆盖其它字段。
4. 支持 `disable_notify` 控制通知，并记录审计与调试日志。

### 2.2 请求参数


| 位置  | 参数             | 是否必填 | 说明                                     |
| ------- | ------------------ | ---------- | ------------------------------------------ |
| Query | `disable_notify` | 否       | `true/false`，默认 `false`               |
| Body  | `cardId`         | 是       | 需要更新的卡片 ID                        |
| Body  | `status`         | 是       | 新的状态值（如`TODO`、`进行中`、`完成`） |

### 2.3 示例

```bash
curl -X POST http://localhost:8000/api/v2/ai/cards/modify \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <TOKEN>" \
  -H "X-Requested-With: XMLHttpRequest" \
  -d '{
    "cardId": "card-id-456",
    "status": "进行中"
  }'
```

**成功响应**：返回更新后的卡片 JSON，可在 `properties` 中看到新的状态值。
**常见错误**：缺少 `cardId`/`status`（400）、状态属性未找到（400）、权限不足（403）、卡片不存在（404）。

---

## 3. 快速对比


| 接口               | 主要作用 | 关键字段                         | 典型用途              |
| -------------------- | ---------- | ---------------------------------- | ----------------------- |
| `/ai/cards/create` | 新建卡片 | `boardId`、`title`、`properties` | 批量导入、AI 生成任务 |
| `/ai/cards/modify` | 更新状态 | `cardId`、`status`               | AI 根据进度自动流转   |
