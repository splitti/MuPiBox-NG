package audio

import "time"

// Simulated deliberately produces no sound. Duration is a fixed test value.
type Simulated struct { position float64; paused bool; loaded bool; last time.Time }
func (*Simulated) Name()string{return "simulated"}
func (s *Simulated) Load(_ string,_ int)error{s.position=0;s.paused=false;s.loaded=true;s.last=time.Now();return nil}
func (s *Simulated) advance(){if s.loaded&&!s.paused{s.position+=time.Since(s.last).Seconds();if s.position>180{s.position=180}};s.last=time.Now()}
func (s *Simulated) Pause(v bool)error{s.advance();s.paused=v;return nil}
func (*Simulated) Volume(int)error{return nil}
func (s *Simulated) Seek(v float64)error{s.advance();s.position=v;return nil}
func (s *Simulated) Snapshot()(Snapshot,error){s.advance();return Snapshot{Position:s.position,Duration:180,Paused:s.paused,Ended:s.loaded&&s.position>=180},nil}
func (s *Simulated) Stop()error{s.loaded=false;s.position=0;return nil}
func (s *Simulated) Close()error{return s.Stop()}
