package gate

import (
	"cherry-game/examples/demo_cluster/internal/code"
	"cherry-game/examples/demo_cluster/internal/pb"
	sessionKey "cherry-game/examples/demo_cluster/internal/session_key"
	"log/slog"

	cslice "github.com/cherry-game/cherry/extend/slice"
	cstring "github.com/cherry-game/cherry/extend/string"
	cfacade "github.com/cherry-game/cherry/facade"
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

	notLoginRsp = &pb.Int32{
		Value: code.PlayerDenyLogin,
	}
)

// onDataRoute 数据路由规则
//
// 登录逻辑:
// 1.(建立连接)客户端建立连接，服务端对应创建一个agent用于处理玩家消息,actorID == sid
// 2.(用户登录)客户端进行帐号登录验证，通过uid绑定当前sid
// 3.(角色登录)客户端通过'beforeLoginRoutes'中的协议完成角色登录
func onPomeloDataRoute(agent *pomelo.Agent, route *pmessage.Route, msg *pmessage.Message) {
	slog.Info("用户开始链接,路由--》", msg.Route)
	session := pomelo.BuildSession(agent, msg)

	// agent没有"用户登录",且请求不是第一条协议，则踢掉agent，断开连接
	if !session.IsBind() && msg.Route != firstRouteName {
		agent.Kick(notLoginRsp, true)
		return
	}

	if agent.NodeType() == route.NodeType() {
		targetPath := cfacade.NewChildPath(agent.NodeID(), route.HandleName(), session.Sid)
		pomelo.LocalDataRoute(agent, session, route, msg, targetPath)
	} else {
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
