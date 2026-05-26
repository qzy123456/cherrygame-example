// internal/event/sign_in.go
package event

type SignIn struct {
	PlayerId       int64
	ContinuousDays int32
}

func NewSignIn(playerId int64, continuousDays int32) SignIn {
	return SignIn{
		PlayerId:       playerId,
		ContinuousDays: continuousDays,
	}
}

func (SignIn) Name() string {
	return "sign_in"
}

func (s SignIn) UniqueID() int64 {
	return s.PlayerId
}
