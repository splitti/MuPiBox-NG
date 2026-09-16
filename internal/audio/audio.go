package audio

// Backend is serialized by the controller. Only the backend owns audio resources.
type Snapshot struct { Position,Duration float64; Paused,Ended bool }
type Backend interface {
 Name() string
 Load(path string,volume int) error
 Pause(bool) error
 Volume(int) error
 Seek(float64) error
 Snapshot()(Snapshot,error)
 Stop() error
 Close() error
}
