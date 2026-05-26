// nodes/center/module/sign_in/actor_sign_in.go
package sign_in

import (
	"time"

	"cherry-game/examples/demo_cluster/internal/code"
	"cherry-game/examples/demo_cluster/internal/pb"
	"cherry-game/examples/demo_cluster/nodes/center/db"

	clog "github.com/cherry-game/cherry/logger"
	cactor "github.com/cherry-game/cherry/net/actor"
)

type ActorSignIn struct {
	cactor.Base
}

func (p *ActorSignIn) AliasID() string {
	return "sign_in"
}

func (p *ActorSignIn) OnInit() {
	p.Remote().Register("GetSignInRecord", p.GetSignInRecord)
	p.Remote().Register("SignIn", p.SignIn)
}

// GetSignInRecord 获取签到记录
func (p *ActorSignIn) GetSignInRecord(playerId int64) (*pb.SignInResponse, int32) {

	return &pb.SignInResponse{}, code.OK
}

// SignIn 执行签到
func (p *ActorSignIn) SignIn(playerId int64) (*pb.SignInResult, int32) {
	today := time.Now().Format("2006-01-02")

	record := &db.SignInRecord{}
	db.GetSignInRecord(playerId, record)

	// 检查今日是否已签到
	if record.LastSignInDate == today {
		return nil, code.SignInAlready
	}

	// 计算连续签到天数
	continuousDays := int32(1)
	if record.LastSignInDate != "" {
		lastDate, _ := time.Parse("2006-01-02", record.LastSignInDate)
		todayDate, _ := time.Parse("2006-01-02", today)
		if todayDate.Sub(lastDate).Hours() < 48 {
			continuousDays = record.ContinuousDays + 1
		}
	}

	// 获取今日奖励
	reward := getRewardByDays(continuousDays)

	// 更新数据库
	record.PlayerId = playerId
	record.LastSignInDate = today
	record.ContinuousDays = continuousDays
	record.TotalSignInDays++
	record.TodayReward = reward

	if err := db.SaveSignInRecord(record); err != nil {
		clog.Errorf("save sign in record error: %v", err)
		return nil, code.DBError
	}

	return &pb.SignInResult{
		ContinuousDays: continuousDays,
		TodayReward:    reward,
	}, code.OK
}

func getRewardByDays(days int32) int32 {
	rewards := map[int32]int32{
		1:  100,   // 1天：100金币
		3:  500,   // 3天：500金币
		7:  2000,  // 7天：2000金币
		30: 10000, // 30天：10000金币
	}
	for d := days; d >= 1; d-- {
		if r, ok := rewards[d]; ok {
			return r
		}
	}
	return 100
}
