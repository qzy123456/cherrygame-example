package game

import (
	checkCenter "cherry-game/examples/demo_cluster/internal/component/check_center"
	"cherry-game/examples/demo_cluster/internal/data"
	"cherry-game/examples/demo_cluster/nodes/game/db"
	"cherry-game/examples/demo_cluster/nodes/game/module/player"
	"cherry-game/examples/demo_cluster/nodes/game/module/sign_in"

	"github.com/cherry-game/cherry"
	cherrySnowflake "github.com/cherry-game/cherry/extend/snowflake"
	cstring "github.com/cherry-game/cherry/extend/string"
	cherryUtils "github.com/cherry-game/cherry/extend/utils"
	cherryDiscovery "github.com/cherry-game/cherry/net/discovery"
	cherryCron "github.com/cherry-game/components/cron"
	cherryETCD "github.com/cherry-game/components/etcd"
	cherryGops "github.com/cherry-game/components/gops"
)

func Run(profileFilePath, nodeID string) {
	if !cherryUtils.IsNumeric(nodeID) {
		panic("node parameter must is number.")
	}

	//注册etcd服务发现（必须在cherry.Configure()之前调用）
	cherryDiscovery.Register(cherryETCD.New())
	
	// snowflake global id
	serverId, _ := cstring.ToInt64(nodeID)
	cherrySnowflake.SetDefaultNode(serverId)
	
	// 配置cherry引擎
	app := cherry.Configure(profileFilePath, nodeID, false, cherry.Cluster)

	// diagnose
	app.Register(cherryGops.New())
	// 注册调度组件
	app.Register(cherryCron.New())
	// 注册数据配置组件
	app.Register(data.New())
	// 注册检测中心节点组件，确认中心节点启动后，再启动当前节点
	app.Register(checkCenter.New())
	// 注册db组件
	app.Register(db.New())

	app.AddActors(
		&player.ActorPlayers{},
		&sign_in.ActorSignIn{},
	)

	app.Startup()
}
