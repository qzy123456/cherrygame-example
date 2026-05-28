package gate

import (
	"cherry-game/examples/demo_cluster/internal/code"
	"cherry-game/examples/demo_cluster/internal/data"
	"cherry-game/examples/demo_cluster/internal/pb"
	rpcCenter "cherry-game/examples/demo_cluster/internal/rpc/center"
	sessionKey "cherry-game/examples/demo_cluster/internal/session_key"
	"cherry-game/examples/demo_cluster/internal/token"
	"encoding/json"
	"log/slog"

	cstring "github.com/cherry-game/cherry/extend/string"
	cfacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
	cactor "github.com/cherry-game/cherry/net/actor"
	"github.com/cherry-game/cherry/net/parser/pomelo"
	cproto "github.com/cherry-game/cherry/net/proto"
)

var (
	duplicateLoginCode []byte
)

type (
	// ActorAgent 每个网络连接对应一个ActorAgent
	ActorAgent struct {
		cactor.Base
	}
)

func (p *ActorAgent) OnInit() {
	duplicateLoginCode, _ = p.App().Serializer().Marshal(&cproto.I32{
		Value: code.PlayerDuplicateLogin,
	})

	p.Local().Register("login", p.login)
	p.Local().Register("privateChat", p.privateChat)
	p.Local().Register("privateChatJson", p.privateChatJSON)
	p.Remote().Register("setSession", p.setSession)
}

func (p *ActorAgent) setSession(req *pb.StringKeyValue) {
	slog.Info("设置用户session,请求信息--》", req.Value, req.Key)
	if req.Key == "" {
		return
	}
	if agent, ok := pomelo.GetAgent(p.ActorID(), 0); ok {
		agent.Session().Set(req.Key, req.Value)
	}
}

// login 用户登录，验证帐号 (*pb.LoginResponse, int32)
func (p *ActorAgent) login(session *cproto.Session, req *pb.LoginRequest) {
	slog.Info("用户登录请求信息--》", req)
	agent, found := pomelo.GetAgent(p.ActorID(), session.Uid)
	if !found {
		return
	}

	// 验证token
	userToken, errCode := p.validateToken(req.Token)
	if code.IsFail(errCode) {
		agent.Response(session, errCode)
		return
	}

	// 验证pid是否配置
	sdkRow := data.SdkConfig.Get(userToken.PID)
	if sdkRow == nil {
		agent.ResponseCode(session, code.PIDError, true)
		return
	}

	// 根据token带来的sdk参数，从中心节点获取uid
	uid, errCode := rpcCenter.GetUID(p.App(), sdkRow.SdkId, userToken.PID, userToken.OpenID)
	if uid == 0 || code.IsFail(errCode) {
		agent.ResponseCode(session, code.AccountBindFail, true)
		return
	}

	oldAgent, err := pomelo.Bind(session.Sid, uid)
	if err != nil {
		agent.ResponseCode(session, code.AccountBindFail, true)
		clog.Warn(err)
		return
	}

	// 挤掉之前的agent
	if oldAgent != nil {
		oldAgent.Kick(duplicateLoginCode, true)
	}

	p.checkGateSession(uid)

	agent.Session().Set(sessionKey.ServerID, cstring.ToString(req.ServerId))
	agent.Session().Set(sessionKey.PID, cstring.ToString(userToken.PID))
	agent.Session().Set(sessionKey.OpenID, userToken.OpenID)

	response := &pb.LoginResponse{
		Uid:    uid,
		Pid:    userToken.PID,
		OpenId: userToken.OpenID,
	}

	agent.Response(session, response)

	// 注册用户位置到 Center 服
	p.registerUserLocation(uint64(uid))
}

func (p *ActorAgent) validateToken(base64Token string) (*token.Token, int32) {
	userToken, ok := token.DecodeToken(base64Token)
	if !ok {
		return nil, code.AccountTokenValidateFail
	}

	platformRow := data.SdkConfig.Get(userToken.PID)
	if platformRow == nil {
		return nil, code.PIDError
	}

	statusCode, ok := token.Validate(userToken, platformRow.Salt)
	if !ok {
		return nil, statusCode
	}

	return userToken, code.OK
}

func (p *ActorAgent) checkGateSession(uid cfacade.UID) {
	rsp := &cproto.PomeloKick{
		Uid:    uid,
		Reason: duplicateLoginCode,
	}

	// 遍历其他网关节点，挤掉旧的agent
	members := p.App().Discovery().ListByType(p.App().NodeType(), p.App().NodeID())
	for _, member := range members {
		// user是gate.go里自定义的agentActorID
		actorPath := cfacade.NewPath(member.GetNodeID(), "user")
		p.Call(actorPath, pomelo.KickFuncName, rsp)
	}
}

// onSessionClose  当agent断开时，关闭对应的ActorAgent
func (p *ActorAgent) onSessionClose(agent *pomelo.Agent) {
	session := agent.Session()
	serverId := session.GetString(sessionKey.ServerID)
	if serverId == "" {
		return
	}

	// 通知game节点关闭session
	childId := cstring.ToString(session.Uid)
	if childId != "" {
		targetPath := cfacade.NewChildPath(serverId, "player", childId)
		p.Call(targetPath, "sessionClose", nil)
	}

	// 从 Center 服移除用户位置
	p.removeUserLocation(uint64(session.Uid))

	// 自己退出
	p.Exit()
	clog.Infof("sessionClose path = %s", p.Path())
}

// registerUserLocation 注册用户位置到 Center 服
func (p *ActorAgent) registerUserLocation(uid uint64) {
	gateNodeID := p.App().NodeID()
	targetPath := rpcCenter.GetTargetPath(p.App(), ".location")

	req := &pb.Int64String{
		Key:   int64(uid),
		Value: gateNodeID,
	}

	rsp := &pb.Int32{}
	errCode := p.App().ActorSystem().CallWait(".system", targetPath, "SetUserLocation", req, rsp)
	if code.IsFail(errCode) {
		clog.Warnf("Register user location failed. [uid = %d, gateNodeID = %s, error = %d]", uid, gateNodeID, errCode)
	} else {
		clog.Debugf("Register user location success. [uid = %d, gateNodeID = %s]", uid, gateNodeID)
	}
}

// removeUserLocation 从 Center 服移除用户位置
func (p *ActorAgent) removeUserLocation(uid uint64) {
	targetPath := rpcCenter.GetTargetPath(p.App(), ".location")

	req := &pb.Int64{
		Value: int64(uid),
	}

	rsp := &pb.Int32{}
	errCode := p.App().ActorSystem().CallWait(".system", targetPath, "RemoveUserLocation", req, rsp)
	if code.IsFail(errCode) {
		clog.Warnf("Remove user location failed. [uid = %d, error = %d]", uid, errCode)
	} else {
		clog.Debugf("Remove user location success. [uid = %d]", uid)
	}
}

// privateChat 处理私聊请求
func (p *ActorAgent) privateChat(session *cproto.Session, req *pb.PrivateChatRequest) {
	// 获取发送者信息
	senderUID := uint64(session.Uid)
	targetUID := uint64(req.TargetUid)
	content := req.Content

	clog.Debugf("Private chat request. [sender = %d, target = %d, content = %s]", senderUID, targetUID, content)

	// 查询目标用户所在的 Gate 节点
	targetGateNode, errCode := p.queryUserLocation(targetUID)
	if errCode != code.OK {
		clog.Warnf("Query user location failed. [targetUID = %d, error = %d]", targetUID, errCode)
		p.responsePrivateChat(session, code.PlayerNotOnline, "目标用户不在线")
		return
	}

	// 判断是否在当前 Gate
	currentGateNode := p.App().NodeID()
	if targetGateNode == currentGateNode {
		// 在同一 Gate，直接发送
		errCode := sendToLocalUser(p.App(), targetUID, senderUID, content)
		if errCode != code.OK {
			clog.Warnf("Send to local user failed. [targetUID = %d, error = %d]", targetUID, errCode)
			p.responsePrivateChat(session, code.PlayerNotOnline, "发送失败")
			return
		}
	} else {
		// 在其他 Gate，通过 RPC 转发
		errCode := forwardToRemoteGate(p.App(), targetGateNode, targetUID, senderUID, content)
		if errCode != code.OK {
			clog.Warnf("Forward to remote gate failed. [gate = %s, targetUID = %d, error = %d]", targetGateNode, targetUID, errCode)
			p.responsePrivateChat(session, code.PlayerNotOnline, "发送失败")
			return
		}
	}

	// 发送成功
	p.responsePrivateChat(session, code.OK, "发送成功")
}

// privateChatJSON 处理私聊请求（JSON格式）
func (p *ActorAgent) privateChatJSON(session *cproto.Session, req interface{}) {
	type PrivateChatReq struct {
		TargetUid uint64 `json:"targetUid"`
		Content   string `json:"content"`
	}

	var chatReq PrivateChatReq

	var jsonData []byte
	switch v := req.(type) {
	case []byte:
		jsonData = v
	case string:
		jsonData = []byte(v)
	default:
		clog.Warnf("Unsupported request type: %T", req)
		p.responsePrivateChat(session, code.Error, "请求格式错误")
		return
	}

	if err := json.Unmarshal(jsonData, &chatReq); err != nil {
		clog.Warnf("Parse private chat JSON request failed. [error = %v, data = %s]", err, string(jsonData))
		p.responsePrivateChat(session, code.Error, "请求格式错误")
		return
	}

	// 获取发送者信息
	senderUID := uint64(session.Uid)
	targetUID := chatReq.TargetUid
	content := chatReq.Content

	clog.Debugf("Private chat JSON request. [sender = %d, target = %d, content = %s]", senderUID, targetUID, content)

	// 查询目标用户所在的 Gate 节点
	targetGateNode, errCode := p.queryUserLocation(targetUID)
	if errCode != code.OK {
		clog.Warnf("Query user location failed. [targetUID = %d, error = %d]", targetUID, errCode)
		p.responsePrivateChat(session, code.PlayerNotOnline, "目标用户不在线")
		return
	}

	// 判断是否在当前 Gate
	currentGateNode := p.App().NodeID()
	if targetGateNode == currentGateNode {
		// 在同一 Gate，直接发送
		errCode := sendToLocalUser(p.App(), targetUID, senderUID, content)
		if errCode != code.OK {
			clog.Warnf("Send to local user failed. [targetUID = %d, error = %d]", targetUID, errCode)
			p.responsePrivateChat(session, code.PlayerNotOnline, "发送失败")
			return
		}
	} else {
		// 在其他 Gate，通过 RPC 转发
		errCode := forwardToRemoteGate(p.App(), targetGateNode, targetUID, senderUID, content)
		if errCode != code.OK {
			clog.Warnf("Forward to remote gate failed. [gate = %s, targetUID = %d, error = %d]", targetGateNode, targetUID, errCode)
			p.responsePrivateChat(session, code.PlayerNotOnline, "发送失败")
			return
		}
	}

	// 发送成功
	p.responsePrivateChat(session, code.OK, "发送成功")
}

// queryUserLocation 查询用户位置
func (p *ActorAgent) queryUserLocation(uid uint64) (string, int32) {
	targetGateNode, errCode := rpcCenter.GetUserLocation(p.App(), uid)
	if errCode != code.OK {
		return "", errCode
	}
	if targetGateNode == "" {
		return "", code.PlayerNotOnline
	}
	return targetGateNode, code.OK
}

// responsePrivateChat 回复私聊请求
func (p *ActorAgent) responsePrivateChat(session *cproto.Session, errCode int32, message string) {
	agent, found := pomelo.GetAgent(p.ActorID(), session.Uid)
	if !found {
		return
	}

	response := &pb.PrivateChatResponse{
		Code:    errCode,
		Message: message,
	}
	agent.Response(session, response)
}
