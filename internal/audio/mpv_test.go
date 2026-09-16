package audio

import (
 "encoding/binary"
 "os"
 "os/exec"
 "path/filepath"
 "testing"
 "time"
)
func TestMPVRealDecode(t *testing.T){
 if _,err:=exec.LookPath("mpv");err!=nil{t.Skip("mpv not installed; real decoding must be verified on target")}
 path:=filepath.Join(t.TempDir(),"tone.wav")
 // Five seconds of signed PCM silence: valid generated audio, no copyrighted fixtures.
 data:=make([]byte,44+8000*2*5);copy(data,"RIFF");binary.LittleEndian.PutUint32(data[4:],uint32(len(data)-8));copy(data[8:],"WAVEfmt ");binary.LittleEndian.PutUint32(data[16:],16);binary.LittleEndian.PutUint16(data[20:],1);binary.LittleEndian.PutUint16(data[22:],1);binary.LittleEndian.PutUint32(data[24:],8000);binary.LittleEndian.PutUint32(data[28:],16000);binary.LittleEndian.PutUint16(data[32:],2);binary.LittleEndian.PutUint16(data[34:],16);copy(data[36:],"data");binary.LittleEndian.PutUint32(data[40:],uint32(len(data)-44));if e:=os.WriteFile(path,data,0600);e!=nil{t.Fatal(e)}
 m:=&MPV{NullOutput:true};defer m.Close()
 if e:=m.Load(path,20);e!=nil{t.Fatal(e)}
 time.Sleep(150*time.Millisecond)
 s,e:=m.Snapshot();if e!=nil{t.Fatal(e)};if s.Duration<4.9||s.Duration>5.1||s.Paused{t.Fatalf("bad snapshot %+v",s)}
 if e=m.Pause(true);e!=nil{t.Fatal(e)};s,e=m.Snapshot();if e!=nil||!s.Paused{t.Fatalf("pause: %+v %v",s,e)}
 if e=m.Seek(2);e!=nil{t.Fatal(e)};if e=m.Volume(35);e!=nil{t.Fatal(e)}
 if e=m.Pause(false);e!=nil{t.Fatal(e)}
 if e=m.Seek(4.9);e!=nil{t.Fatal(e)}
 deadline:=time.Now().Add(3*time.Second);for time.Now().Before(deadline){s,e=m.Snapshot();if e!=nil{t.Fatal(e)};if s.Ended{break};time.Sleep(50*time.Millisecond)};if !s.Ended{t.Fatal("EOF not detected")}
 if e=m.Load(filepath.Join(t.TempDir(),"missing.mp3"),20);e==nil{t.Fatal("missing audio reported as playing")}
}
