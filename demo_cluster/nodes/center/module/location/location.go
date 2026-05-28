package location

import (
	"fmt"
	"sync"

	"cherry-game/examples/demo_cluster/internal/pb"

	cherryLogger "github.com/cherry-game/cherry/logger"
	cactor "github.com/cherry-game/cherry/net/actor"
)

// ActorLocation 用户位置管理Actor
type ActorLocation struct {
	cactor.Base
	sync.RWMutex
	locationMap map[uint64]string // uid -> gateNodeID
}

func (p *ActorLocation) AliasID() string {
	return "location"
}

// OnInit 初始化用户位置管理
func (p *ActorLocation) OnInit() {
	p.locationMap = make(map[uint64]string)

	// 注册RPC方法
	p.Remote().Register("GetUserLocation", p.GetUserLocation)
	p.Remote().Register("SetUserLocation", p.SetUserLocation)
	p.Remote().Register("RemoveUserLocation", p.RemoveUserLocation)
	p.Remote().Register("GetUserLocationBatch", p.GetUserLocationBatch)

	cherryLogger.Info("UserLocation RPC methods registered")
}

// SetUserLocation 设置用户位置
func (p *ActorLocation) SetUserLocation(req *pb.Int64String) *pb.Int32 {
	if req == nil {
		return &pb.Int32{Value: 1}
	}

	uid := uint64(req.Key)
	gateNodeID := req.Value

	p.Lock()
	p.locationMap[uid] = gateNodeID
	currentMap := fmt.Sprintf("%v", p.locationMap)
	p.Unlock()
	cherryLogger.Debugf("Set user location. [uid = %d, gateNodeID = %s]", uid, gateNodeID)
	cherryLogger.Debugf("Current locationMap: %s", currentMap)
	return &pb.Int32{Value: 0}
}

// GetUserLocation 获取用户位置
func (p *ActorLocation) GetUserLocation(req *pb.Int64) (*pb.String, int32) {
	if req == nil {
		return &pb.String{Value: ""}, 1
	}

	uid := uint64(req.Value)
	cherryLogger.Debugf("获取到请求的用户id： [uid = %d]", uid)
	cherryLogger.Debugf("当前map: %s", fmt.Sprintf("%v", p.locationMap))
	p.RLock()
	nodeID, ok := p.locationMap[uid]
	p.RUnlock()
	if !ok {
		cherryLogger.Debugf("Get user location miss. [uid = %d]", uid)
		return &pb.String{Value: ""}, 0
	}
	cherryLogger.Debugf("Get user location hit. [uid = %d, gateNodeID = %s]", uid, nodeID)
	return &pb.String{Value: nodeID}, 0
}

// RemoveUserLocation 移除用户位置
func (p *ActorLocation) RemoveUserLocation(req *pb.Int64) *pb.Int32 {
	if req == nil {
		return &pb.Int32{Value: 1}
	}

	uid := uint64(req.Value)
	p.Lock()
	delete(p.locationMap, uid)
	p.Unlock()
	cherryLogger.Debugf("Remove user location. [uid = %d]", uid)
	return &pb.Int32{Value: 0}
}

// GetUserLocationBatch 批量获取用户位置
func (p *ActorLocation) GetUserLocationBatch(req *pb.Int64List) *pb.Int64StringList {
	if req == nil || len(req.List) == 0 {
		return &pb.Int64StringList{}
	}

	p.RLock()
	defer p.RUnlock()

	result := &pb.Int64StringList{}
	for _, uid := range req.List {
		if nodeID, ok := p.locationMap[uint64(uid)]; ok {
			result.List = append(result.List, &pb.Int64String{
				Key:   uid,
				Value: nodeID,
			})
		}
	}
	return result
}
