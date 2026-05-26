package web

import (
	checkCenter "cherry-game/examples/demo_cluster/internal/component/check_center"
	"cherry-game/examples/demo_cluster/internal/data"
	"cherry-game/examples/demo_cluster/nodes/web/controller"
	"cherry-game/examples/demo_cluster/nodes/web/sdk"

	"github.com/cherry-game/cherry"
	cherryFile "github.com/cherry-game/cherry/extend/file"
	cherryDiscovery "github.com/cherry-game/cherry/net/discovery"
	cherryCron "github.com/cherry-game/components/cron"
	cherryETCD "github.com/cherry-game/components/etcd"
	cherryGin "github.com/cherry-game/components/gin"
	"github.com/gin-gonic/gin"
)

func Run(profileFilePath, nodeID string) {
	//注册etcd服务发现（必须在cherry.Configure()之前调用）
	cherryDiscovery.Register(cherryETCD.New())
	// 配置cherry引擎,加载profile配置文件
	app := cherry.Configure(profileFilePath, nodeID, false, cherry.Cluster)

	// 注册调度组件
	app.Register(cherryCron.New())

	// 注册检查中心服是否启动组件
	app.Register(checkCenter.New())

	// 注册数据配表组件
	app.Register(data.New())

	// 加载http server组件
	app.Register(httpServerComponent(app.Address()))

	// 加载sdk逻辑
	sdk.Init(app)

	// 启动cherry引擎
	app.Startup()
}

func httpServerComponent(addr string) *cherryGin.Component {
	gin.SetMode(gin.DebugMode)

	// new http server
	httpServer := cherryGin.NewHttp("http_server", addr)
	httpServer.Use(cherryGin.Cors())

	// http server使用gin组件搭建，这里增加一个RecoveryWithZap中间件
	httpServer.Use(cherryGin.RecoveryWithZap(true))

	// 映射h5客户端静态文件到static目录，例如：http://127.0.0.1/static/protocol.js
	httpServer.Static("/static", "./static/")

	// 加载./view目录的html模板文件(详情查看gin文档)
	viewFiles := cherryFile.WalkFiles("./view/", ".html")
	if len(viewFiles) < 1 {
		panic("view files not found.")
	}
	httpServer.LoadHTMLFiles(viewFiles...)

	//注册 controller
	httpServer.Register(new(controller.Controller))

	return httpServer
}
