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

// getSourcePath 获取源路径
func getSourcePath(app cfacade.IApplication) string {
	return app.NodeID() + ".system"
}

// Ping 访问center节点，确认center已启动
func Ping(app cfacade.IApplication) bool {
	nodeID := GetCenterNodeID(app)
	if nodeID == "" {
		return false
	}

	rsp := &pb.Bool{}
	targetPath := nodeID + opsActor
	errCode := app.ActorSystem().CallWait(getSourcePath(app), targetPath, ping, nil, rsp)
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
	errCode := app.ActorSystem().CallWait(getSourcePath(app), targetPath, registerDevAccount, req, rsp)
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
	errCode := app.ActorSystem().CallWait(getSourcePath(app), targetPath, getDevAccount, req, rsp)
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
	errCode := app.ActorSystem().CallWait(getSourcePath(app), targetPath, getUID, req, rsp)
	if code.IsFail(errCode) {
		clog.Warnf("[GetUID] errCode = %v", errCode)
		return 0, errCode
	}

	return rsp.Value, code.OK
}

func GetCenterNodeID(app cfacade.IApplication) string {
	list := app.Discovery().ListByType(centerType)
	gateList := app.Discovery().ListByType("gate")
	clog.Infof("获取center列表 %v", list)
	clog.Infof("获取网关列表 %v", gateList)
	gameList := app.Discovery().ListByType("game")
	clog.Infof("获取game列表 %v", gameList)
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

	errCode := app.ActorSystem().CallWait(getSourcePath(app), targetPath, "GetSignInRecord", playerId, rsp)
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

	errCode := app.ActorSystem().CallWait(getSourcePath(app), targetPath, "SignIn", playerId, rsp)
	if code.IsFail(errCode) {
		clog.Warnf("[SignIn] playerId = %v, errCode = %v", playerId, errCode)
		return nil, errCode
	}

	return rsp, code.OK
}

const (
	locationActor = ".location"
)

// GetUserLocation 查询用户所在的 Gate 节点
func GetUserLocation(app cfacade.IApplication, uid uint64) (string, int32) {
	targetPath := GetTargetPath(app, locationActor)
	req := &pb.Int64{Value: int64(uid)}
	rsp := &pb.String{}

	errCode := app.ActorSystem().CallWait(getSourcePath(app), targetPath, "GetUserLocation", req, rsp)
	clog.Infof("[GetUserLocation] targetPath=%s uid=%v errCode=%v rsp=%v", targetPath, uid, errCode, rsp)
	if code.IsFail(errCode) {
		clog.Warnf("[GetUserLocation] uid = %v, targetPath = %s, errCode = %v", uid, targetPath, errCode)
		return "", errCode
	}

	if rsp.Value == "" {
		clog.Infof("[GetUserLocation] uid = %v not found in center, targetPath = %s", uid, targetPath)
		return "", code.PlayerNotOnline
	}

	clog.Infof("[GetUserLocation] uid = %v -> gate = %s", uid, rsp.Value)
	return rsp.Value, code.OK
}

// PushToUser 推送消息给指定用户
func PushToUser(app cfacade.IApplication, targetGateNode string, uid uint64, fromUid uint64, content string) int32 {
	if targetGateNode == "" {
		clog.Warnf("[PushToUser] empty target gate node for uid = %v", uid)
		return code.PlayerNotOnline
	}

	targetPath := targetGateNode + ".remote"

	req := &pb.PrivateChatPush{
		TargetUid: uid,
		FromUid:   fromUid,
		Content:   content,
		Timestamp: 0,
	}

	rsp := &pb.Int32{}
	errCode := app.ActorSystem().CallWait(getSourcePath(app), targetPath, "forwardPrivateChat", req, rsp)
	if code.IsFail(errCode) {
		clog.Warnf("[PushToUser] gate = %s, uid = %v, errCode = %v", targetGateNode, uid, errCode)
		return errCode
	}

	return code.OK
}
