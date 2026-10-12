---
name: sync-upstream-gateway
description: 将本仓库的 Go gateway 实现与上游 LobeHub 参考实现（device-gateway / agent-gateway）以及应用侧客户端协议进行对齐。当需要同步上游协议、追平最新实现、或对齐到指定 LobeHub tag/commit 时调用。
---

# Sync Upstream Gateway Implementation

用于将 `orvilo-gateway` 的 Go 实现与上游 LobeHub gateway 参考实现的协议行为对齐。

## 何时使用

- 任务要求"同步上游""对齐协议""追平 LobeHub 最新实现"时
- 任务指定一个 LobeHub tag 或 commit，要求 Go 实现对齐到该版本时
- 上游 device-gateway / agent-gateway 有协议变更，需要评估并同步到本仓库时

## 背景

本仓库是上游 TypeScript 实现的 Go 重写，**协议基线在 `README.md` 的 "Alignment Target" 章节固定为两个 commit**：

- `device-gateway` at `<commit>`
- `agent-gateway` at `<commit>`

上游参考仓库（`AGENTS.md` 约定）：

| 上游 | 仓库 | 默认分支 | 用途 |
| --- | --- | --- | --- |
| Device Gateway | `github.com/lobehub-biz/device-gateway` | `main` | 设备网关协议参考 |
| Agent Gateway | `github.com/lobehub-biz/agent-gateway` | `master` | Agent 网关协议参考 |
| 应用侧 | `github.com/lobehub/lobehub` | `main` | 客户端调用约定参考 |

应用侧重点核对路径：

- `packages/device-gateway-client`
- `packages/agent-gateway-client`
- `src/server/agent-hono/handlers`

参考 checkout 位置（**仅用于参考，不属于本项目源码**）：

- `device-gateway/`、`agent-gateway/` — 项目根目录下，已被 `.gitignore` 忽略
- `lobehub/` — 项目根目录的**上一级**（`../lobehub/`），避免污染本项目工作区

## 版本约定（来自 AGENTS.md）

- 默认遵循 README 固定的协议基线；任务指定 LobeHub tag 或 commit 时以该版本为准。
- 只有明确要求时才跟踪 `main` 或 `canary`。
- 面向开发者和用户描述最终行为、接口、配置与约束；仅在 changelog 或迁移指南中记录演变过程。

## 同步流程

### 1. 确定目标版本

两条路径，由任务决定：

- **路径 A — 对齐到指定 tag/commit（默认）**：从任务中获取目标 tag 或 commit。
- **路径 B — 追平上游最新**：分别取 `device-gateway` 的 `main` HEAD、`agent-gateway` 的 `master` HEAD。仅在任务明确要求跟踪最新时采用。

若任务未指定且未要求追平，**默认沿用 README 固定基线，不主动同步**；应先向用户确认意图。

### 2. 准备上游参考 checkout

在项目根目录下 clone（目录已被 `.gitignore` 忽略）：

```bash
git clone https://github.com/lobehub-biz/device-gateway device-gateway
git clone https://github.com/lobehub-biz/agent-gateway agent-gateway
git clone --depth 1 https://github.com/lobehub/lobehub ../lobehub   # 应用侧，clone 到上一级，按需
```

按路径切换到目标版本：

- 路径 A：`cd device-gateway && git checkout <target-commit-or-tag>`，agent-gateway 同理。
- 路径 B：保持默认分支并 `git pull` 到最新。

### 3. 读取当前基线

从 `README.md` 与 `README.zh-CN.md` 的 "Alignment Target" 章节读取当前固定的两个 commit，作为 diff 的起点。

### 4. Diff 协议变化

对比 **当前基线 → 目标版本** 之间影响公开协议的变化，重点关注：

- HTTP 路由（路径、方法、状态码）
- 请求 / 响应 JSON schema（字段名、类型、可空性、默认值）
- WebSocket 消息格式与事件名
- 鉴权行为（`SERVICE_TOKEN`、JWKS/JWT 校验）
- 路由语义、超时行为
- 类型定义（`src/types.ts`）

**明确排除**（README "Alignment Target" 声明，不同步）：

- admin APIs
- metrics
- Durable Object 存储或 WebSocket hibernation
- 多实例协调
- geo reporting、admin reporting

实现位置对照：

| 上游文件 | 本仓库位置 |
| --- | --- |
| `device-gateway/src/*.ts` | `device-gateway-go/internal/gateway/` |
| `agent-gateway/src/*.ts` | `agent-gateway-go/internal/gateway/` |

### 5. 核对应用侧客户端协议

在 `../lobehub/` 中核对客户端如何调用 gateway，确认本仓库暴露的接口与客户端约定一致：

- `packages/device-gateway-client`
- `packages/agent-gateway-client`
- `src/server/agent-hono/handlers`

若发现客户端依赖了本仓库未实现的接口，列入待对齐清单并按优先级处理。

### 6. 更新 Go 实现

按 diff 结果更新 `device-gateway-go` / `agent-gateway-go` 的 `internal/gateway/` 实现。遵循现有代码风格与内存单实例设计，**不引入**：

- Cloudflare Workers / Durable Objects 绑定
- Redis、PostgreSQL、NATS 等外部运行时依赖

### 7. 更新协议基线

若已对齐到新版本，更新 `README.md` 与 `README.zh-CN.md` 的 "Alignment Target" 两个 commit 为目标版本的实际 commit。

### 8. 验证

```bash
# 根模块（统一二进制）
go build -o gateway ./cmd/gateway
go test ./...

# 各子模块独立验证
cd agent-gateway-go && go test ./... && cd ..
cd device-gateway-go && go test ./... && cd ..
```

针对新增/变更的协议行为补充或更新测试用例。

### 9. 记录变化

在 `docs/release_notes/` 或 changelog 中记录本次协议对齐的内容：对齐到的版本、变更的接口/消息、行为差异、迁移注意点。**只描述最终行为与约束，不记录中间探索过程**。

## 完成判据

- README 基线 commit 已更新（若版本变更）
- `go test ./...` 及两个子模块测试通过
- `go build -o gateway ./cmd/gateway` 成功
- 变更已写入 release notes / changelog
- 未引入被排除的平台特性或外部依赖
