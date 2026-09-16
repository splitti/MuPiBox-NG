package player

import (
	"sync"
	"time"
)

type MemoryPlayer struct {
	mu sync.Mutex
	st PlayerStatus

	// trackDurations is optional; if empty we use st.Duration for all tracks.
	trackDurations []int

	ticker *time.Ticker
	done   chan struct{}
}

func NewMemoryPlayer() *MemoryPlayer {
	p := &MemoryPlayer{
		st: PlayerStatus{
			State: StatePaused,
			Mode:  ModeAudiobookChapters,

			Series: "Benjamin Blümchen",
			Title:  "Folge 12 – Der Zoo brennt",

			Track:      1,
			TrackCount: 20,

			Position: 0,
			Duration: 420, // 7 min demo

			Cover: "/covers/placeholder.png",

			Volume: 40,
			Muted:  false,

			CanSeek:      true,
			CanSkipTrack: true,
			CanSkipTime:  true,
		},
		trackDurations: make([]int, 20),
		done:           make(chan struct{}),
	}

	// Demo: vary durations a bit
	for i := 0; i < len(p.trackDurations); i++ {
		p.trackDurations[i] = 300 + (i * 10) // 5:00, 5:10, ...
	}

	p.ticker = time.NewTicker(1 * time.Second)
	go p.loop()

	return p
}

func (p *MemoryPlayer) Close() {
	close(p.done)
	p.ticker.Stop()
}

func (p *MemoryPlayer) loop() {
	for {
		select {
		case <-p.done:
			return
		case <-p.ticker.C:
			p.mu.Lock()
			if p.st.State == StatePlaying {
				p.st.Position++
				if p.st.Position >= p.currentDurationLocked() {
					// auto-next
					p.nextLocked()
				}
			}
			p.mu.Unlock()
		}
	}
}

func (p *MemoryPlayer) Status() PlayerStatus {
	p.mu.Lock()
	defer p.mu.Unlock()

	// ensure Duration matches current track
	p.st.Duration = p.currentDurationLocked()
	return p.st
}

func (p *MemoryPlayer) Play() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.st.State == StateStopped {
		p.st.Position = 0
	}
	p.st.State = StatePlaying
}

func (p *MemoryPlayer) Pause() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.st.State == StatePlaying {
		p.st.State = StatePaused
	}
}

func (p *MemoryPlayer) Toggle() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.st.State == StatePlaying {
		p.st.State = StatePaused
		return
	}
	if p.st.State == StateStopped {
		p.st.Position = 0
	}
	p.st.State = StatePlaying
}

func (p *MemoryPlayer) Next() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.nextLocked()
}

func (p *MemoryPlayer) Prev() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.st.Position > 3 {
		// common UX: if you’re a few seconds in, prev restarts
		p.st.Position = 0
		return
	}

	if p.st.TrackCount <= 0 {
		return
	}
	p.st.Track--
	if p.st.Track < 1 {
		p.st.Track = 1
	}
	p.st.Position = 0
	p.st.Duration = p.currentDurationLocked()
}

func (p *MemoryPlayer) SetTrack(nr int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.st.TrackCount <= 0 {
		return
	}
	if nr < 1 {
		nr = 1
	}
	if nr > p.st.TrackCount {
		nr = p.st.TrackCount
	}

	p.st.Track = nr
	p.st.Position = 0
	p.st.Duration = p.currentDurationLocked()
}

func (p *MemoryPlayer) Seek(pos int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.st.CanSeek {
		return
	}
	if pos < 0 {
		pos = 0
	}
	d := p.currentDurationLocked()
	if pos > d {
		pos = d
	}
	p.st.Position = pos
	p.st.Duration = d
}

func (p *MemoryPlayer) Skip(seconds int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.st.CanSkipTime {
		return
	}
	pos := p.st.Position + seconds
	if pos < 0 {
		pos = 0
	}
	d := p.currentDurationLocked()
	if pos > d {
		pos = d
	}
	p.st.Position = pos
	p.st.Duration = d
}

func (p *MemoryPlayer) SetVolume(level int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if level < 0 {
		level = 0
	}
	if level > 100 {
		level = 100
	}
	p.st.Volume = level
	// if user sets volume > 0, usually unmute
	if level > 0 {
		p.st.Muted = false
	}
}

func (p *MemoryPlayer) Mute() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.st.Muted = true
}

func (p *MemoryPlayer) Unmute() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.st.Muted = false
}

func (p *MemoryPlayer) ToggleMute() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.st.Muted = !p.st.Muted
}

func (p *MemoryPlayer) nextLocked() {
	if p.st.TrackCount <= 0 {
		p.st.State = StateStopped
		p.st.Position = 0
		return
	}
	p.st.Track++
	if p.st.Track > p.st.TrackCount {
		// stop at end
		p.st.Track = p.st.TrackCount
		p.st.State = StatePaused
		p.st.Position = p.currentDurationLocked()
		return
	}
	p.st.Position = 0
	p.st.Duration = p.currentDurationLocked()
}

func (p *MemoryPlayer) currentDurationLocked() int {
	if p.st.TrackCount <= 0 {
		if p.st.Duration <= 0 {
			return 0
		}
		return p.st.Duration
	}
	idx := p.st.Track - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(p.trackDurations) || p.trackDurations[idx] <= 0 {
		if p.st.Duration <= 0 {
			return 0
		}
		return p.st.Duration
	}
	return p.trackDurations[idx]
}
