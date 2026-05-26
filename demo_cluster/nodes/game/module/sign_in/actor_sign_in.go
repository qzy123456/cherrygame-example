// nodes/game/module/sign_in/actor_sign_in.go
package sign_in

import (
	"cherry-game/examples/demo_cluster/internal/code"
	"cherry-game/examples/demo_cluster/internal/event"
	"cherry-game/examples/demo_cluster/internal/pb"
	rpcCenter "cherry-game/examples/demo_cluster/internal/rpc/center"
	"cherry-game/examples/demo_cluster/nodes/game/module/online"

	"github.com/cherry-game/cherry/net/parser/pomelo"
	cproto "github.com/cherry-game/cherry/net/proto"
)

type ActorSignIn struct {
	pomelo.ActorBase
}

func (p *ActorSignIn) OnInit() {
	p.Local().Register("SignIn", p.SignIn)
	p.Remote().Register("GetSignInStatus", p.GetSignInStatus)
}

// SignIn 玩家签到（本地调用）
func (p *ActorSignIn) SignIn(session *cproto.Session, req *pb.SignInRequest) {
	playerId := online.GetPlayerId(session.Uid)
	if playerId == 0 {
		p.Response(session, &pb.SignInResponse{Code: code.PlayerNotLogin})
		return
	}

	// 调用 Center 节点执行签到
	result, errCode := rpcCenter.SignIn(p.App(), playerId)
	if code.IsFail(errCode) {
		p.Response(session, &pb.SignInResponse{Code: errCode})
		return
	}

	// 发布签到事件
	signInEvent := event.NewSignIn(playerId, result.ContinuousDays)
	p.PostEvent(&signInEvent)

	// 返回响应
	p.Response(session, &pb.SignInResponse{
		Code:           code.OK,
		ContinuousDays: result.ContinuousDays,
		TodayReward:    result.TodayReward,
	})
}

// GetSignInStatus 获取签到状态（远程调用）
func (p *ActorSignIn) GetSignInStatus(playerId int64) *pb.SignInStatus {
	record, errCode := rpcCenter.GetSignInRecord(p.App(), playerId)
	if code.IsFail(errCode) {
		return &pb.SignInStatus{
			PlayerId:       playerId,
			ContinuousDays: 0,
			HasSignedToday: false,
		}
	}

	return &pb.SignInStatus{
		PlayerId:       playerId,
		ContinuousDays: record.ContinuousDays,
		HasSignedToday: false,
	}
}
