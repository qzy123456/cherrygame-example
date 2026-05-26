package event

type PlayerCreate struct {
	PlayerID   int64
	PlayerName string
	Gender     int32
}

func NewPlayerCreate(playerID int64, playerName string, gender int32) PlayerCreate {
	event := PlayerCreate{
		PlayerID:   playerID,
		PlayerName: playerName,
		Gender:     gender,
	}
	return event
}

func (PlayerCreate) Name() string {
	return PlayerCreateKey
}

func (p PlayerCreate) UniqueID() int64 {
	return p.PlayerID
}
