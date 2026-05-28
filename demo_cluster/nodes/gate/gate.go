package gate

import (
	"encoding/binary"
	"log/slog"
	"time"

	checkCenter "cherry-game/examples/demo_cluster/internal/component/check_center"
	"cherry-game/examples/demo_cluster/internal/data"

	"github.com/cherry-game/cherry"
	cfacade "github.com/cherry-game/cherry/facade"
	cconnector "github.com/cherry-game/cherry/net/connector"
	cherryDiscovery "github.com/cherry-game/cherry/net/discovery"
	"github.com/cherry-game/cherry/net/parser/pomelo"
	"github.com/cherry-game/cherry/net/parser/simple"
	cherryETCD "github.com/cherry-game/components/etcd"
	cherryGops "github.com/cherry-game/components/gops"
)

// Run 运行gate节点
// gate 主要用于对外提供网络连接、管理用户连接、消息转发
func Run(profileFilePath, nodeID string) {
	//注册etcd服务发现（必须在cherry.Configure()之前调用）
	cherryDiscovery.Register(cherryETCD.New())

	// 创建一个cherry实例
	app := cherry.Configure(
		profileFilePath,
		nodeID,
		true,
		cherry.Cluster,
	)
	// 设置网络数据包解析器
	netParser := buildPomeloParser(app)
	app.SetNetParser(netParser)

	app.Register(cherryGops.New())
	// 注册检则中心服组件，用于检则中心服是否先启动
	app.Register(checkCenter.New())
	// 注册数据配表组件，具体详见data-config的使用方法和参数配置
	app.Register(data.New())

	// 注册远程调用处理 Actor
	app.AddActors(&ActorRemote{})

	app.Register()

	//启动cherry引擎
	app.Startup()
}

var appInstance cfacade.IApplication

func buildPomeloParser(app *cherry.AppBuilder) cfacade.INetParser {
	// 保存应用实例供路由函数使用
	appInstance = app
	// 使用pomelo网络数据包解析器
	agentActor := pomelo.NewActor("user")
	//todo 创建一个tcp监听，用于client/robot压测机器人连接网关tcp
	// 由于这里是写死的、启动多个网关的时候有问题、暂时注释
	//agentActor.AddConnector(cconnector.NewTCP(":10011"))
	//再创建一个websocket监听，用于h5客户端建立连接
	agentActor.AddConnector(cconnector.NewWS(app.Address()))
	//当有新连接创建Agent时，启动一个自定义(ActorAgent)的子actor
	agentActor.SetOnNewAgent(func(newAgent *pomelo.Agent) {
		slog.Info("用户开始链接？或者gate启动？")
		childActor := &ActorAgent{}
		newAgent.AddOnClose(childActor.onSessionClose)
		agentActor.Child().Create(newAgent.SID(), childActor) // actorID == sid
	})

	// 设置数据路由函数
	agentActor.SetOnDataRoute(onPomeloDataRoute)

	return agentActor
}

// 构建简单的网络数据包解析器
func buildSimpleParser(app *cherry.AppBuilder) cfacade.INetParser {
	agentActor := simple.NewActor("user")
	//todo 压测时候打开tcp监听，用于client/robot压测机器人连接网关tcp
	//agentActor.AddConnector(cconnector.NewTCP(":10011"))
	agentActor.AddConnector(cconnector.NewWS(app.Address()))

	agentActor.SetOnNewAgent(func(newAgent *simple.Agent) {
		childActor := &ActorAgent{}
		//newAgent.AddOnClose(childActor.onSessionClose)
		agentActor.Child().Create(newAgent.SID(), childActor)
	})

	// 设置大头&小头
	agentActor.SetEndian(binary.LittleEndian)
	// 设置心跳时间
	agentActor.SetHeartbeatTime(60 * time.Second)
	// 设置积压消息数量
	agentActor.SetWriteBacklog(64)

	// 设置数据路由函数
	//agentActor.SetOnDataRoute(onPomeloDataRoute)

	// 设置消息节点路由(建议配合data-config组件进行使用)
	// mid = 1 的消息路由到  gate节点.user的Actor.login函数上
	agentActor.AddNodeRoute(1, &simple.NodeRoute{
		NodeType: "gate",
		ActorID:  "user",
		FuncName: "login",
	})

	return agentActor
}
