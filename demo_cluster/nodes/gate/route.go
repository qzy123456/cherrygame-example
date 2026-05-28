package gate

import (
	"cherry-game/examples/demo_cluster/internal/code"
	"cherry-game/examples/demo_cluster/internal/pb"
	rpcCenter "cherry-game/examples/demo_cluster/internal/rpc/center"
	sessionKey "cherry-game/examples/demo_cluster/internal/session_key"
	"encoding/json"
	"fmt"
	"log/slog"

	cslice "github.com/cherry-game/cherry/extend/slice"
	cstring "github.com/cherry-game/cherry/extend/string"
	cfacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
	"github.com/cherry-game/cherry/net/parser/pomelo"
	pmessage "github.com/cherry-game/cherry/net/parser/pomelo/message"
	cproto "github.com/cherry-game/cherry/net/proto"
)

var (
	// 客户端连接后，必需先执行第一条协议，进行token验证后，才能进行后续的逻辑
	firstRouteName = "gate.user.login"

	// 角色进入游戏时的前三个协议
	beforeLoginRoutes = []string{
		"game.player.select", //查询玩家角色
		"game.player.create", //玩家创建角色
		"game.player.enter",  //玩家角色进入游戏
	}

	// Gate 服本地处理的协议（不需要转发到 Game 服）
	gateLocalRoutes = []string{
		"gate.user.login",           // 用户登录
		"gate.user.privateChat",     // 私聊（Protobuf格式）
		"gate.user.privateChatJson", // 私聊（JSON格式）
	}

	notLoginRsp = &pb.Int32{
		Value: code.PlayerDenyLogin,
	}
)

// onDataRoute 数据路由规则
//
// 登录逻辑:
// 1.(建立连接)客户端建立连接，服务端对应创建agent用于处理玩家消息,actorID == sid
// 2.(用户登录)客户端进行帐号登录验证，通过uid绑定当前sid
// 3.(角色登录)客户端通过'beforeLoginRoutes'中的协议完成角色登录
func onPomeloDataRoute(agent *pomelo.Agent, route *pmessage.Route, msg *pmessage.Message) {
	slog.Info("用户开始链接", "route", msg.Route)
	slog.Info("消息数据长度", "len", len(msg.Data), "data", fmt.Sprintf("%x", msg.Data[:min(len(msg.Data), 30)]))
	session := pomelo.BuildSession(agent, msg)

	// agent没有"用户登录",且请求不是第一条协议，则踢掉agent，断开连接
	if !session.IsBind() && msg.Route != firstRouteName {
		slog.Warn("Session not bind", "route", msg.Route)
		agent.Kick(notLoginRsp, true)
		return
	}

	slog.Info("Session bind status", "isBind", session.IsBind(), "uid", session.Uid)

	if agent.NodeType() == route.NodeType() {
		// Gate 服本地处理的消息
		// 私聊JSON消息特殊处理
		if msg.Route == "gate.user.privateChatJson" {
			slog.Info("Received privateChatJson request, processing...")
			handlePrivateChatJSON(agent, session, msg)
			return
		}

		targetPath := cfacade.NewChildPath(agent.NodeID(), route.HandleName(), session.Sid)
		pomelo.LocalDataRoute(agent, session, route, msg, targetPath)
	} else {
		// 需要转发到 Game 服的消息
		gameNodeRoute(agent, session, route, msg)
	}
}

// gameNodeRoute 实现agent路由消息到游戏节点
func gameNodeRoute(agent *pomelo.Agent, session *cproto.Session, route *pmessage.Route, msg *pmessage.Message) {
	slog.Info("路由分发--》", msg.Route)
	if !session.IsBind() {
		return
	}

	// 如果agent没有完成"角色登录",则禁止转发到game节点
	if !session.Contains(sessionKey.PlayerID) {
		// 如果不是角色登录协议则踢掉agent
		if found := cslice.StringInSlice(msg.Route, beforeLoginRoutes); !found {
			agent.Kick(notLoginRsp, true)
			return
		}
	}
	// 判断又没有传过来的game服务器id-也就是区服id
	serverId := session.GetString(sessionKey.ServerID)
	if serverId == "" {
		return
	}
	// 每个用户设置一个子actor
	childId := cstring.ToString(session.Uid)
	// 构建目标 Actor 路径,- childId ：将用户 ID 转为字符串，作为子 Actor 的 ID
	//- targetPath ：构建目标路径，格式为 {serverId}.{handleName}.{childId}
	//- 例如： 10001.player.13 （10001 服的玩家 13）
	targetPath := cfacade.NewChildPath(serverId, route.HandleName(), childId)
	//- 将消息转发到指定的 Game 节点
	// - 参数说明：
	// - agent ：当前连接的 Agent
	// - session ：会话信息
	// - route ：路由信息（包含目标方法名）
	// - msg ：消息内容
	// - serverId ：目标 Game 节点 ID
	// - targetPath ：目标 Actor 路径
	// 完整流程：
	// 客户端发送消息
	//    ↓
	// Gate 节点接收
	// 		↓
	// 从 Session 获取 serverId
	// 		↓
	// 构建目标路径：{serverId}.{handleName}.{uid}
	// 		↓
	// 调用 ClusterLocalDataRoute 转发
	// 		↓
	// 消息到达对应的 Game 节点
	// 		↓
	// 路由到具体的 actorPlayer 实例
	pomelo.ClusterLocalDataRoute(agent, session, route, msg, serverId, targetPath)
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// handlePrivateChatJSON 处理私聊请求（JSON格式）
func handlePrivateChatJSON(agent *pomelo.Agent, session *cproto.Session, msg *pmessage.Message) {
	type PrivateChatReq struct {
		TargetUid uint64 `json:"targetUid"`
		Content   string `json:"content"`
	}

	var chatReq PrivateChatReq

	// 使用框架的 Serializer 来解析数据
	var reqMap map[string]interface{}
	err := appInstance.Serializer().Unmarshal(msg.Data, &reqMap)
	if err != nil {
		clog.Warnf("Serializer unmarshal failed. [error = %v, data length = %d, data hex = %x]", err, len(msg.Data), msg.Data[:min(len(msg.Data), 20)])

		// 如果Serializer解析失败，尝试直接JSON解析（可能是pomelo-client发送的原始JSON）
		if jsonErr := json.Unmarshal(msg.Data, &chatReq); jsonErr != nil {
			clog.Warnf("JSON unmarshal also failed. [error = %v]", jsonErr)
			responsePrivateChat(agent, session, code.Error, "请求格式错误")
			return
		}
	} else {
		// 从map中提取数据
		if targetUidVal, ok := reqMap["targetUid"]; ok {
			switch v := targetUidVal.(type) {
			case float64:
				chatReq.TargetUid = uint64(v)
			case int:
				chatReq.TargetUid = uint64(v)
			case uint64:
				chatReq.TargetUid = v
			}
		}
		if contentVal, ok := reqMap["content"]; ok {
			chatReq.Content = contentVal.(string)
		}
	}

	clog.Debugf("Private chat parsed. [sender = %d, target = %d, content = %s]", session.Uid, chatReq.TargetUid, chatReq.Content)

	senderUID := uint64(session.Uid)
	targetUID := chatReq.TargetUid
	content := chatReq.Content

	clog.Debugf("Private chat JSON request. [sender = %d, target = %d, content = %s]", senderUID, targetUID, content)

	// 查询目标用户所在的 Gate 节点
	targetGateNode, errCode := rpcCenter.GetUserLocation(appInstance, targetUID)
	if errCode != code.OK || targetGateNode == "" {
		clog.Warnf("Query user location failed. [targetUID = %d, gate = %s, error = %d]", targetUID, targetGateNode, errCode)
		responsePrivateChat(agent, session, code.PlayerNotOnline, "目标用户不在线")
		return
	}

	// 判断是否在当前 Gate
	currentGateNode := agent.NodeID()
	if targetGateNode == currentGateNode {
		// 在同一 Gate，直接发送
		errCode := sendToLocalUser(appInstance, targetUID, senderUID, content)
		if errCode != code.OK {
			clog.Warnf("Send to local user failed. [targetUID = %d, error = %d]", targetUID, errCode)
			responsePrivateChat(agent, session, code.PlayerNotOnline, "发送失败")
			return
		}
	} else {
		// 在其他 Gate，通过 RPC 转发
		errCode := forwardToRemoteGate(appInstance, targetGateNode, targetUID, senderUID, content)
		if errCode != code.OK {
			clog.Warnf("Forward to remote gate failed. [gate = %s, targetUID = %d, error = %d]", targetGateNode, targetUID, errCode)
			responsePrivateChat(agent, session, code.PlayerNotOnline, "发送失败")
			return
		}
	}

	responsePrivateChat(agent, session, code.OK, "发送成功")
}

// responsePrivateChat 回复私聊请求
func responsePrivateChat(agent *pomelo.Agent, session *cproto.Session, errCode int32, message string) {
	response := map[string]interface{}{
		"code":    errCode,
		"message": message,
	}
	data, _ := json.Marshal(response)
	agent.Response(session, data)
}

// sendToLocalUser 发送消息给本地用户
func sendToLocalUser(app cfacade.IApplication, targetUID uint64, senderUID uint64, content string) int32 {
	pushData := map[string]interface{}{
		"fromUid": senderUID,
		"content": content,
	}
	data, _ := json.Marshal(pushData)

	targetAgent, found := pomelo.GetAgent("", int64(targetUID))
	if !found {
		return code.PlayerNotOnline
	}

	targetAgent.Push("onPrivateChatJson", data)
	return code.OK
}

// forwardToRemoteGate 转发消息到其他 Gate 节点
func forwardToRemoteGate(app cfacade.IApplication, targetGateNode string, targetUID uint64, senderUID uint64, content string) int32 {
	// 调用目标 Gate 的 RPC 方法转发消息
	return rpcCenter.PushToUser(app, targetGateNode, targetUID, senderUID, content)
}
