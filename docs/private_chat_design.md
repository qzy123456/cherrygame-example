# 私聊功能架构设计方案

## 一、方案对比

| 方案 | 优点 | 缺点 | 适用场景 |
|-----|------|------|---------|
| **Center 服处理** | 逻辑简单，统一管理 | 单点瓶颈，延迟高 | 小规模、低并发 |
| **Gate 服处理** | 低延迟，分布式 | 需要跨 gate 通信 | 大规模、高并发 |
| **混合方案（推荐）** | 兼顾低延迟和可扩展性 | 实现稍复杂 | 生产环境首选 |

## 二、推荐方案：混合架构

### 架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                      Center 服                              │
│  ┌─────────────────────────────────────────────────────┐    │
│  │         用户位置映射表                               │    │
│  │    locationMap: map[uid] → gateNodeID               │    │
│  └─────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────┘
                              ↑ ↓
              ┌───────────────┴───────────────┐
              ↓                               ↓
┌────────────────────────┐    ┌────────────────────────┐
│       Gate-A 服        │    │       Gate-B 服        │
│  ┌──────────────────┐  │    │  ┌──────────────────┐  │
│  │ 用户连接管理      │  │    │  │ 用户连接管理      │  │
│  │ 消息发送/转发     │  │    │  │ 消息发送/转发     │  │
│  └──────────────────┘  │    │  └──────────────────┘  │
└────────────────────────┘    └────────────────────────┘
              ↓                               ↓
    ┌─────────┴─────────┐          ┌─────────┴─────────┐
    │ 玩家 A │ 玩家 C   │          │ 玩家 B │ 玩家 D   │
    └───────────────────┘          └───────────────────┘
```

### 核心职责划分

| 节点 | 职责 |
|-----|------|
| **Center 服** | 维护用户位置映射表，提供位置查询服务 |
| **Gate 服** | 处理私聊消息发送，跨 gate 转发 |

## 三、关键代码实现

### 1. 用户位置管理（Center 服）

```go
package center

import (
    "sync"
    "github.com/cherry-game/cherry/facade"
)

// UserLocation 管理用户所在的 Gate 节点位置
type UserLocation struct {
    sync.RWMutex
    locationMap map[uint64]string // uid → gateNodeID
}

var Location = &UserLocation{
    locationMap: make(map[uint64]string),
}

// Set 设置用户位置
func (u *UserLocation) Set(uid uint64, gateNodeID string) {
    u.Lock()
    u.locationMap[uid] = gateNodeID
    u.Unlock()
}

// Get 获取用户位置
func (u *UserLocation) Get(uid uint64) (string, bool) {
    u.RLock()
    nodeID, ok := u.locationMap[uid]
    u.RUnlock()
    return nodeID, ok
}

// Remove 移除用户位置
func (u *UserLocation) Remove(uid uint64) {
    u.Lock()
    delete(u.locationMap, uid)
    u.Unlock()
}

// RegisterRPC 注册 RPC 方法供其他节点调用
func (u *UserLocation) RegisterRPC() {
    cherry.Remote().Register("GetUserLocation", u.GetUserLocationRPC)
    cherry.Remote().Register("SetUserLocation", u.SetUserLocationRPC)
    cherry.Remote().Register("RemoveUserLocation", u.RemoveUserLocationRPC)
}

func (u *UserLocation) GetUserLocationRPC(uid uint64) (string, error) {
    nodeID, ok := u.Get(uid)
    if !ok {
        return "", errors.New("user not found")
    }
    return nodeID, nil
}

func (u *UserLocation) SetUserLocationRPC(uid uint64, gateNodeID string) error {
    u.Set(uid, gateNodeID)
    return nil
}

func (u *UserLocation) RemoveUserLocationRPC(uid uint64) error {
    u.Remove(uid)
    return nil
}
```

### 2. 私聊发送逻辑（Gate 服）

```go
package gate

import (
    "errors"
    "github.com/cherry-game/cherry/facade"
    "github.com/cherry-game/cherry/net/pomelo"
)

// sendPrivateMessage 发送私聊消息
func (p *ActorAgent) sendPrivateMessage(targetUID uint64, message string) error {
    // 1. 查询目标用户所在的 Gate 节点
    targetGateNode, err := p.queryUserLocation(targetUID)
    if err != nil {
        return errors.New("target user not online")
    }
    
    // 2. 判断是否在当前 Gate
    currentGateNode := cherry.App().NodeID()
    if targetGateNode == currentGateNode {
        // 在同一 Gate，直接发送
        return p.sendToLocalUser(targetUID, message)
    } else {
        // 在其他 Gate，通过 RPC 转发
        return p.forwardToRemoteGate(targetGateNode, targetUID, message)
    }
}

// queryUserLocation 查询用户位置
func (p *ActorAgent) queryUserLocation(uid uint64) (string, error) {
    var result string
    err := p.Call("gc-center.location", "GetUserLocation", uid, &result)
    return result, err
}

// sendToLocalUser 发送消息给本地用户
func (p *ActorAgent) sendToLocalUser(targetUID uint64, message string) error {
    agent, ok := pomelo.GetAgentWithUID(targetUID)
    if !ok {
        return errors.New("agent not found")
    }
    
    // 推送私聊消息
    data := &pb.PrivateMessage{
        FromUID: p.Uid(),
        Content: message,
    }
    agent.Push("onPrivateMessage", data)
    return nil
}

// forwardToRemoteGate 转发消息到其他 Gate 节点
func (p *ActorAgent) forwardToRemoteGate(gateNodeID string, targetUID uint64, message string) error {
    // 构建目标路径：{gateNodeID}.user.forwardPrivateMessage
    targetPath := cherry.Facade.NewPath(gateNodeID, "user")
    return p.Call(targetPath, "forwardPrivateMessage", targetUID, message)
}

// forwardPrivateMessage 接收其他 Gate 转发的消息（注册为远程方法）
func (p *ActorAgent) forwardPrivateMessage(targetUID uint64, message string) error {
    return p.sendToLocalUser(targetUID, message)
}

// 在 OnInit 中注册远程方法
func (p *ActorAgent) OnInit() {
    p.Local().Register("login", p.login)
    p.Remote().Register("setSession", p.setSession)
    p.Remote().Register("forwardPrivateMessage", p.forwardPrivateMessage) // 新增
}
```

### 3. 用户位置更新时机（Gate 服）

```go
// 用户登录时注册位置
func (p *ActorAgent) onLoginSuccess(uid uint64) {
    gateNodeID := cherry.App().NodeID()
    p.Call("gc-center.location", "SetUserLocation", uid, gateNodeID)
}

// 用户退出时移除位置
func (p *ActorAgent) onSessionClose() {
    uid := p.Uid()
    p.Call("gc-center.location", "RemoveUserLocation", uid)
}
```

## 四、完整流程

### 场景：玩家 A 在 Gate-A，玩家 B 在 Gate-B，A 发送私聊给 B

```
玩家 A 发送私聊请求
        ↓
Gate-A 接收请求
        ↓
Gate-A 查询 Center 服：用户 B 的位置
        ↓
Center 返回：用户 B 在 Gate-B
        ↓
Gate-A 判断：不在同一 Gate
        ↓
Gate-A RPC 调用 Gate-B.forwardPrivateMessage
        ↓
Gate-B 接收转发请求
        ↓
Gate-B 查找用户 B 的 Agent
        ↓
Gate-B 推送消息给用户 B
        ↓
用户 B 收到私聊消息
```

## 五、设计要点

### 1. 容错机制

```go
// 消息发送失败重试
func (p *ActorAgent) sendWithRetry(targetGateNode, targetUID, message string, maxRetries int) error {
    var err error
    for i := 0; i < maxRetries; i++ {
        err = p.forwardToRemoteGate(targetGateNode, targetUID, message)
        if err == nil {
            return nil
        }
        time.Sleep(time.Millisecond * 100)
    }
    return err
}
```

### 2. 性能优化

```go
// Gate 本地缓存用户位置（定期刷新）
type LocationCache struct {
    sync.RWMutex
    cache      map[uint64]string
    expireTime time.Time
}

func (c *LocationCache) Get(uid uint64) (string, bool) {
    c.RLock()
    defer c.RUnlock()
    
    // 检查缓存是否过期
    if time.Since(c.expireTime) > 30*time.Second {
        return "", false
    }
    
    nodeID, ok := c.cache[uid]
    return nodeID, ok
}
```

### 3. 批量查询支持

```go
// Center 服支持批量查询
func (u *UserLocation) GetBatch(uidList []uint64) map[uint64]string {
    u.RLock()
    defer u.RUnlock()
    
    result := make(map[uint64]string)
    for _, uid := range uidList {
        if nodeID, ok := u.locationMap[uid]; ok {
            result[uid] = nodeID
        }
    }
    return result
}
```

## 六、消息协议定义

```protobuf
// 私聊请求
message PrivateMessageRequest {
    uint64 target_uid = 1;   // 目标用户 ID
    string content = 2;      // 消息内容
}

// 私聊消息
message PrivateMessage {
    uint64 from_uid = 1;     // 发送者 ID
    string content = 2;      // 消息内容
    int64 timestamp = 3;     // 时间戳
}

// 系统通知（用户离线等）
message SystemNotification {
    int32 code = 1;          // 错误码
    string message = 2;      // 提示信息
}
```

## 七、总结

### 优势

1. **低延迟**：同 Gate 直接发送，无需中转
2. **高可用**：分布式设计，无单点故障
3. **易扩展**：支持水平扩展多个 Gate 节点
4. **易维护**：职责清晰，便于定位问题

### 部署建议

1. **Center 服**：至少部署 2 台，做高可用
2. **Gate 服**：根据在线用户量横向扩展
3. **缓存策略**：Gate 本地缓存用户位置，30 秒刷新一次

### 监控指标

| 指标 | 说明 |
|-----|------|
| 私聊消息延迟 | 从发送到接收的时间 |
| 跨 Gate 转发成功率 | 跨节点消息发送成功率 |
| 用户位置命中率 | 本地缓存命中比例 |
| Center 查询 QPS | 用户位置查询频率 |