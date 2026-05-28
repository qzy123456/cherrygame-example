# 私聊功能问题排查与修复总结

## 问题概述

本次修复解决了 Cherry 框架集群模式下私聊功能的多个问题，包括：
- 前端消息序列化问题
- RPC 调用返回值问题  
- 跨 Gate 节点通信问题
- Actor 注册和路径问题

---

## 问题修复清单

### 1. 前端 Protobuf 序列化问题

**问题描述**：前端发送消息时 JSON 序列化代码被注释，导致消息格式错误

**修复文件**：`nodes/web/static/pomelo-client-protobuf.js:279`

**修复内容**：取消注释 JSON 序列化代码

```javascript
// 修复前（被注释）
// msg = Protocol.strencode(JSON.stringify(msg));

// 修复后
msg = Protocol.strencode(JSON.stringify(msg));
```

---

### 2. GetUserLocation RPC 返回值问题

**问题描述**：`ActorLocation.GetUserLocation` 方法只返回 `*pb.String`，缺少错误码，导致 RPC 调用无法正确解析响应

**修复文件**：`nodes/center/module/location/location.go`

**修复内容**：修改方法签名，返回 `(*pb.String, int32)`

```go
// 修复前
func (p *ActorLocation) GetUserLocation(req *pb.Int64) *pb.String

// 修复后
func (p *ActorLocation) GetUserLocation(req *pb.Int64) (*pb.String, int32)
```

---

### 3. GetAgent 参数错误

**问题描述**：调用 `pomelo.GetAgent(app.NodeID(), int64(targetUID))` 时，第一个参数错误地传递了节点ID，导致无法找到用户连接

**修复文件**：`nodes/gate/route.go:235`、`nodes/gate/actor_agent.go:328`

**修复内容**：第一个参数传空字符串，让函数使用 UID 查找

```go
// 修复前
pomelo.GetAgent(app.NodeID(), int64(targetUID))

// 修复后
pomelo.GetAgent("", int64(targetUID))
```

---

### 4. 前后端数据格式不一致

**问题描述**：服务器推送 Protobuf 格式数据，但前端期望 JSON 格式

**修复方案**：统一使用 JSON 格式推送

**修改文件**：
- `nodes/gate/route.go`: 推送路由改为 `onPrivateChatJson`
- `nodes/web/view/chat_gate1.html`: 监听 `onPrivateChatJson`，移除 `JSON.parse()`
- `nodes/web/view/chat_gate2.html`: 同上

```go
// 修复前
targetAgent.Push("onPrivateChat", data)

// 修复后
targetAgent.Push("onPrivateChatJson", data)
```

```javascript
// 修复前
function onPrivateChat(data) {
    data = JSON.parse(data);
    ...
}

// 修复后
function onPrivateChat(data) {
    // 数据已自动解析为对象
    ...
}
```

---

### 5. RPC 源路径无效

**问题描述**：`sourcePath` 被硬编码为 `.system`，缺少 nodeID，导致 RPC 调用无法正确路由

**修复文件**：`internal/rpc/center/center.go`

**修复内容**：将常量改为动态函数

```go
// 修复前
const sourcePath = ".system"

// 修复后
func getSourcePath(app cfacade.IApplication) string {
    return app.NodeID() + ".system"
}
```

---

### 6. 跨 Gate 通信问题

**问题描述**：`forwardPrivateChat` 方法注册在每个连接的子 Actor 上，但远程调用无法定位到具体的子 Actor

**修复方案**：创建全局 `ActorRemote` 处理跨节点调用

**新增文件**：`nodes/gate/actor_remote.go`

```go
type ActorRemote struct {
    cactor.Base
}

func (p *ActorRemote) AliasID() string {
    return "remote"
}

func (p *ActorRemote) OnInit() {
    p.Remote().Register("forwardPrivateChat", p.forwardPrivateChat)
}
```

**注册方式**：`nodes/gate/gate.go:46`

```go
app.AddActors(&ActorRemote{})
```

**目标路径修改**：`internal/rpc/center/center.go:187`

```go
// 修复前
targetPath := targetGateNode + ".user"

// 修复后
targetPath := targetGateNode + ".remote"
```

---

## 跨 Gate 私聊流程

```
用户A(gate-2) ──发送消息──→ gate-2.route
                              │
                              ▼
                    查询用户B位置(center)
                              │
                              ▼
                    用户B在 gate-1
                              │
                              ▼
                    RPC调用 gate-1.remote.forwardPrivateChat
                              │
                              ▼
                    gate-1.ActorRemote.forwardPrivateChat
                              │
                              ▼
                    根据UID找到用户B的连接
                              │
                              ▼
                    推送消息 onPrivateChatJson
                              │
                              ▼
                    用户B(gate-1) 收到消息
```

---

## 修改文件清单

| 文件路径 | 修改类型 | 问题描述 |
|----------|----------|----------|
| `nodes/web/static/pomelo-client-protobuf.js` | 修改 | 前端消息序列化 |
| `nodes/center/module/location/location.go` | 修改 | RPC 返回值格式 |
| `nodes/gate/route.go` | 修改 | GetAgent 参数、推送路由 |
| `nodes/gate/actor_agent.go` | 修改 | GetAgent 参数、移除冗余方法 |
| `nodes/gate/actor_remote.go` | 新增 | 跨节点 RPC 处理 |
| `nodes/gate/gate.go` | 修改 | 注册 ActorRemote |
| `internal/rpc/center/center.go` | 修改 | 源路径、目标路径 |
| `nodes/web/view/chat_gate1.html` | 修改 | 监听事件、数据解析 |
| `nodes/web/view/chat_gate2.html` | 修改 | 监听事件、数据解析 |

---

## 关键知识点

### Actor 路径格式

在 Cherry 框架中，Actor 的路径格式为：`nodeID.actorType`

- **nodeID**：节点唯一标识，如 `gc-gate-1`
- **actorType**：Actor 别名，通过 `AliasID()` 方法指定

### Actor 类型对比

| Actor | 创建方式 | 路径格式 | 用途 |
|-------|----------|----------|------|
| `ActorAgent` | 每个连接创建一个 | `gc-gate-X.user@sid` | 处理当前连接的本地消息 |
| `ActorRemote` | 节点启动时创建一个 | `gc-gate-X.remote` | 处理跨节点的远程调用 |

### 前端消息解析逻辑

`pomelo-client-protobuf.js` 的 `deCompose` 函数会根据路由名判断解析方式：

```javascript
if (decodeIO_decoder && decodeIO_decoder.lookup(route)) {
    // Protobuf 格式
    return decodeIO_decoder.build(route).decode(msg.body);
} else if (isJsonRoute(route)) {
    // JSON 格式（路由以 json 结尾）
    return JSON.parse(Protocol.strdecode(msg.body));
}
```

---

## 后续建议

1. **日志增强**：在关键路径添加更多日志，便于问题排查
2. **错误处理**：增强错误处理逻辑，返回更详细的错误信息
3. **代码规范**：统一 RPC 调用的路径构建方式
4. **测试覆盖**：添加单元测试和集成测试

---

## 作者信息

- 创建时间：2026-05-28
- 适用版本：Cherry Game Engine
- 项目：demo_cluster
