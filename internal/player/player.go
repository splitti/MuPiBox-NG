package player

type Player interface {
	Play(id string) error
	Pause() error
	Next() error
	SetShuffle(on bool) error
}
