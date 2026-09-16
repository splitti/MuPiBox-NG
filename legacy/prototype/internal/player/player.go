package player

type Player interface {
	Status() PlayerStatus

	Play()
	Pause()
	Toggle()

	Next()
	Prev()
	SetTrack(nr int) // 1-based

	Seek(pos int)      // seconds within current track
	Skip(seconds int)  // +/- seconds

	SetVolume(level int) // 0..100
	Mute()
	Unmute()
	ToggleMute()
}
