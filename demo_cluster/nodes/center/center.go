package center

import (
	"cherry-game/examples/demo_cluster/internal/data"
	"cherry-game/examples/demo_cluster/nodes/center/db"
	"cherry-game/examples/demo_cluster/nodes/center/module/account"
	"cherry-game/examples/demo_cluster/nodes/center/module/ops"
	"cherry-game/examples/demo_cluster/nodes/center/module/sign_in"

	"github.com/cherry-game/cherry"
	cherryDiscovery "github.com/cherry-game/cherry/net/discovery"
	cherryCron "github.com/cherry-game/components/cron"
	cherryETCD "github.com/cherry-game/components/etcd"
)

func Run(profileFilePath, nodeID string) {
	//注册etcd服务发现（必须在cherry.Configure()之前调用）
	cherryDiscovery.Register(cherryETCD.New())

	app := cherry.Configure(
		profileFilePath,
		nodeID,
		false,
		cherry.Cluster,
	)
	//注册组件
	app.Register(cherryCron.New())
	app.Register(data.New())
	app.Register(db.New())
	//注册模块、可以理解为路由
	app.AddActors(
		&account.ActorAccount{},
		&ops.ActorOps{},
		&sign_in.ActorSignIn{},
	)

	app.Startup()
}
