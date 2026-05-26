package checkCenter

import (
	"time"

	rpcCenter "cherry-game/examples/demo_cluster/internal/rpc/center"

	cherryFacade "github.com/cherry-game/cherry/facade"
	cherryLogger "github.com/cherry-game/cherry/logger"
)

// Component 启动时,检查center节点是否存活
type Component struct {
	cherryFacade.Component
}

func New() *Component {
	return &Component{}
}

func (c *Component) Name() string {
	return "run_check_component"
}

func (c *Component) OnAfterInit() {
	go c.waitCenter()
}

func (c *Component) waitCenter() {
	// 等待应用启动完成（etcd session 需要在应用 running 后才会建立）
	for !c.App().Running() {
		time.Sleep(100 * time.Millisecond)
	}

	// 等待 etcd 发现 center 节点
	// 循环检查直到发现 center 节点
	for {
		nodeID := rpcCenter.GetCenterNodeID(c.App())
		if nodeID != "" {
			cherryLogger.Infof("Found center node. [nodeID = %s]", nodeID)
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	// 检查 center 节点是否存活
	for {
		if rpcCenter.Ping(c.App()) {
			break
		}
		time.Sleep(2 * time.Second)
		cherryLogger.Warn("center node connect fail. retrying in 2 seconds.")
	}
}
