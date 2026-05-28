package gate

import (
	"encoding/json"

	"cherry-game/examples/demo_cluster/internal/pb"

	cfacade "github.com/cherry-game/cherry/facade"
	cactor "github.com/cherry-game/cherry/net/actor"
	"github.com/cherry-game/cherry/net/parser/pomelo"
)

// ActorRemote 处理远程调用的 Actor
type ActorRemote struct {
	cactor.Base
}

func (p *ActorRemote) AliasID() string {
	return "remote"
}

func (p *ActorRemote) OnInit() {
	p.Remote().Register("forwardPrivateChat", p.forwardPrivateChat)
}

// forwardPrivateChat 接收其他 Gate 转发的消息
func (p *ActorRemote) forwardPrivateChat(req *pb.PrivateChatPush) *pb.Int32 {
	errCode := p.sendToLocalUser(uint64(req.TargetUid), uint64(req.FromUid), req.Content)
	return &pb.Int32{Value: errCode}
}

// sendToLocalUser 发送消息给本地用户
func (p *ActorRemote) sendToLocalUser(targetUID, senderUID uint64, content string) int32 {
	// 使用 pomelo.GetAgent 根据 UID 获取 agent
	agent, ok := pomelo.GetAgent("", cfacade.UID(targetUID))
	if !ok {
		return 307 // PlayerNotOnline
	}

	// 构建私聊消息推送（JSON格式）
	pushData := map[string]interface{}{
		"fromUid": senderUID,
		"content": content,
	}
	data, _ := json.Marshal(pushData)

	// 推送消息
	agent.Push("onPrivateChatJson", data)
	return 0 // OK
}
