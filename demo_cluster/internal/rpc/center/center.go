package rpcCenter

import (
	"cherry-game/examples/demo_cluster/internal/code"
	"cherry-game/examples/demo_cluster/internal/pb"

	cfacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
)

// route = 节点类型.节点handler.remote函数

const (
	centerType = "center"
)

const (
	opsActor     = ".ops"
	accountActor = ".account"
	signInActor  = ".sign_in"
)

const (
	ping               = "ping"
	registerDevAccount = "registerDevAccount"
	getDevAccount      = "getDevAccount"
	getUID             = "getUID"
)

const (
	sourcePath = ".system"
)

// Ping 访问center节点，确认center已启动
func Ping(app cfacade.IApplication) bool {
	nodeID := GetCenterNodeID(app)
	if nodeID == "" {
		return false
	}

	rsp := &pb.Bool{}
	targetPath := nodeID + opsActor
	errCode := app.ActorSystem().CallWait(sourcePath, targetPath, ping, nil, rsp)
	if code.IsFail(errCode) {
		return false
	}

	return rsp.Value
}

// RegisterDevAccount 注册帐号
func RegisterDevAccount(app cfacade.IApplication, accountName, password, ip string) int32 {
	req := &pb.DevRegister{
		AccountName: accountName,
		Password:    password,
		Ip:          ip,
	}

	targetPath := GetTargetPath(app, accountActor)
	rsp := &pb.Int32{}
	errCode := app.ActorSystem().CallWait(sourcePath, targetPath, registerDevAccount, req, rsp)
	if code.IsFail(errCode) {
		clog.Warnf("[RegisterDevAccount] accountName = %s, errCode = %v", accountName, errCode)
		return errCode
	}

	return rsp.Value
}

// GetDevAccount 获取帐号信息
func GetDevAccount(app cfacade.IApplication, accountName, password string) int64 {
	req := &pb.DevRegister{
		AccountName: accountName,
		Password:    password,
	}

	targetPath := GetTargetPath(app, accountActor)
	rsp := &pb.Int64{}
	errCode := app.ActorSystem().CallWait(sourcePath, targetPath, getDevAccount, req, rsp)
	if code.IsFail(errCode) {
		clog.Warnf("[GetDevAccount] accountName = %s, errCode = %v", accountName, errCode)
		return 0
	}

	return rsp.Value
}

// GetUID 获取帐号UID
func GetUID(app cfacade.IApplication, sdkId, pid int32, openId string) (cfacade.UID, int32) {
	req := &pb.User{
		SdkId:  sdkId,
		Pid:    pid,
		OpenId: openId,
	}

	targetPath := GetTargetPath(app, accountActor)
	rsp := &pb.Int64{}
	errCode := app.ActorSystem().CallWait(sourcePath, targetPath, getUID, req, rsp)
	if code.IsFail(errCode) {
		clog.Warnf("[GetUID] errCode = %v", errCode)
		return 0, errCode
	}

	return rsp.Value, code.OK
}

func GetCenterNodeID(app cfacade.IApplication) string {
	list := app.Discovery().ListByType(centerType)
	gateList := app.Discovery().ListByType("gate")
	clog.Info("获取网关列表%v", gateList)
	gameList := app.Discovery().ListByType("game")
	clog.Info("获取game列表%v", gameList)
	if len(list) > 0 {
		return list[0].GetNodeID()
	}
	return ""
}

func GetTargetPath(app cfacade.IApplication, actorID string) string {
	nodeID := GetCenterNodeID(app)
	return nodeID + actorID
}

// GetSignInRecord 获取签到记录
func GetSignInRecord(app cfacade.IApplication, playerId int64) (*pb.SignInRecord, int32) {
	targetPath := GetTargetPath(app, signInActor)
	rsp := &pb.SignInRecord{}

	errCode := app.ActorSystem().CallWait(sourcePath, targetPath, "GetSignInRecord", playerId, rsp)
	if code.IsFail(errCode) {
		clog.Warnf("[GetSignInRecord] playerId = %v, errCode = %v", playerId, errCode)
		return nil, errCode
	}

	return rsp, code.OK
}

// SignIn 执行签到
func SignIn(app cfacade.IApplication, playerId int64) (*pb.SignInResult, int32) {
	targetPath := GetTargetPath(app, signInActor)
	rsp := &pb.SignInResult{}

	errCode := app.ActorSystem().CallWait(sourcePath, targetPath, "SignIn", playerId, rsp)
	if code.IsFail(errCode) {
		clog.Warnf("[SignIn] playerId = %v, errCode = %v", playerId, errCode)
		return nil, errCode
	}

	return rsp, code.OK
}
