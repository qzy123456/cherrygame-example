package db

import (
	"github.com/goburrow/cache"
	"time"
)

// SignInRecord 签到记录表
type SignInRecord struct {
	PlayerId        int64  `json:"playerId"`
	LastSignInDate  string `json:"lastSignInDate"`
	ContinuousDays  int32  `json:"continuousDays"`
	TotalSignInDays int32  `json:"totalSignInDays"`
	TodayReward     int32  `json:"todayReward"`
}

var signInCache = cache.New(
	cache.WithMaximumSize(65535),
	cache.WithExpireAfterAccess(120*time.Minute),
)

func GetSignInRecord(playerId int64, record *SignInRecord) error {
	val, found := signInCache.GetIfPresent(playerId)
	if found {
		*record = val.(SignInRecord)
		return nil
	}
	*record = SignInRecord{PlayerId: playerId}
	return nil
}

func SaveSignInRecord(record *SignInRecord) error {
	signInCache.Put(record.PlayerId, *record)
	return nil
}

func loadSignInRecord() {
}
