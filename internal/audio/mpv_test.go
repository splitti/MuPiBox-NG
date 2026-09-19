package audio

import (
 "encoding/binary"
 "os"
 "os/exec"
 "path/filepath"
 "regexp"
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
// firstALSAHardwareDevice returns an mpv --audio-device id for the first
// real ALSA card `aplay -l` reports, or "" if there is none (e.g. the DEV
// LXC has no sound hardware at all -- this test is skipped there and only
// runs for real on Pi/MuPiHAT target hardware).
func firstALSAHardwareDevice(t *testing.T) string {
 t.Helper()
 out,err:=exec.Command("aplay","-l").CombinedOutput()
 if err!=nil{return ""}
 re:=regexp.MustCompile(`(?m)^card (\d+): (\S+) `)
 m:=re.FindStringSubmatch(string(out))
 if m==nil{return ""}
 return "alsa/plughw:CARD="+m[2]+",DEV=0"
}
// TestMPVConcurrentInstancesOnExclusiveDeviceFailClearly reproduces a real
// bug found while hardware-testing Phase 2 on the Pi: a second mpv process
// (e.g. the one-shot TTS/announcement backend) cannot open an exclusive
// alsa "plughw" device while the main player's mpv still holds it (even
// paused). Before this fix, Load() declared success right after the
// "file-loaded" event, missing the "audio output initialization failed"
// end-file/error that only arrives afterwards -- so the caller believed
// playback started while nothing was audible. Load() must now surface a
// clear error in this situation instead of silently lying about success.
func TestMPVConcurrentInstancesOnExclusiveDeviceFailClearly(t *testing.T){
 if _,err:=exec.LookPath("mpv");err!=nil{t.Skip("mpv not installed; real decoding must be verified on target")}
 device:=firstALSAHardwareDevice(t)
 if device==""{t.Skip("no real ALSA hardware card available on this host; verified on real Pi/MuPiHAT hardware")}
 path:=filepath.Join(t.TempDir(),"tone.wav")
 data:=make([]byte,44+8000*2*3);copy(data,"RIFF");binary.LittleEndian.PutUint32(data[4:],uint32(len(data)-8));copy(data[8:],"WAVEfmt ");binary.LittleEndian.PutUint32(data[16:],16);binary.LittleEndian.PutUint16(data[20:],1);binary.LittleEndian.PutUint16(data[22:],1);binary.LittleEndian.PutUint32(data[24:],8000);binary.LittleEndian.PutUint32(data[28:],16000);binary.LittleEndian.PutUint16(data[32:],2);binary.LittleEndian.PutUint16(data[34:],16);copy(data[36:],"data");binary.LittleEndian.PutUint32(data[40:],uint32(len(data)-44))
 if e:=os.WriteFile(path,data,0600);e!=nil{t.Fatal(e)}
 holder:=&MPV{Device:device};defer holder.Close()
 if e:=holder.Load(path,20);e!=nil{t.Skipf("could not hold the real device for this test: %v",e)}
 second:=&MPV{Device:device};defer second.Close()
 if e:=second.Load(path,20);e==nil{t.Fatal("a second mpv instance on a busy exclusive device silently reported success")}
}
// TestMPVSurvivesProcessCrash covers Phase 2 item 12: if the mpv process
// dies unexpectedly (killed, OOM on Pi 3, ...), subsequent calls must
// return a clear error instead of panicking or hanging the controller.
func TestMPVSurvivesProcessCrash(t *testing.T){
 if _,err:=exec.LookPath("mpv");err!=nil{t.Skip("mpv not installed; real decoding must be verified on target")}
 path:=filepath.Join(t.TempDir(),"tone.wav")
 data:=make([]byte,44+8000*2*2);copy(data,"RIFF");binary.LittleEndian.PutUint32(data[4:],uint32(len(data)-8));copy(data[8:],"WAVEfmt ");binary.LittleEndian.PutUint32(data[16:],16);binary.LittleEndian.PutUint16(data[20:],1);binary.LittleEndian.PutUint16(data[22:],1);binary.LittleEndian.PutUint32(data[24:],8000);binary.LittleEndian.PutUint32(data[28:],16000);binary.LittleEndian.PutUint16(data[32:],2);binary.LittleEndian.PutUint16(data[34:],16);copy(data[36:],"data");binary.LittleEndian.PutUint32(data[40:],uint32(len(data)-44))
 if e:=os.WriteFile(path,data,0600);e!=nil{t.Fatal(e)}
 m:=&MPV{NullOutput:true};defer m.Close()
 if e:=m.Load(path,20);e!=nil{t.Fatal(e)}
 if e:=m.cmd.Process.Kill();e!=nil{t.Fatal(e)}
 _,_=m.cmd.Process.Wait()
 if _,e:=m.Snapshot();e==nil{t.Fatal("snapshot after process crash reported success")}
 if e:=m.Pause(true);e==nil{t.Fatal("pause after process crash reported success")}
 // A fresh Load must recover cleanly -- the crash must not wedge the adapter.
 if e:=m.Load(path,20);e!=nil{t.Fatalf("could not recover after a crashed mpv process: %v",e)}
}
func TestMPVRejectsInvalidFileContent(t *testing.T){
 if _,err:=exec.LookPath("mpv");err!=nil{t.Skip("mpv not installed; real decoding must be verified on target")}
 path:=filepath.Join(t.TempDir(),"garbage.mp3")
 if e:=os.WriteFile(path,[]byte("this is not a valid audio file, just some bytes with an mp3 extension"),0600);e!=nil{t.Fatal(e)}
 m:=&MPV{NullOutput:true};defer m.Close()
 if e:=m.Load(path,20);e==nil{t.Fatal("undecodable content reported as playing")}
}
// TestMPVRejectsMissingConfiguredDevice covers the Phase-1 known gap
// (docs/mupihat.md "known gap"): a configured device that mpv's live
// audio-device-list no longer contains must be a clear error, never a
// silent fallback to mpv's own default output.
func TestMPVRejectsMissingConfiguredDevice(t *testing.T){
 if _,err:=exec.LookPath("mpv");err!=nil{t.Skip("mpv not installed; device enumeration must be verified on target")}
 path:=filepath.Join(t.TempDir(),"tone.wav")
 data:=make([]byte,44+8000*2);copy(data,"RIFF");binary.LittleEndian.PutUint32(data[4:],uint32(len(data)-8));copy(data[8:],"WAVEfmt ");binary.LittleEndian.PutUint32(data[16:],16);binary.LittleEndian.PutUint16(data[20:],1);binary.LittleEndian.PutUint16(data[22:],1);binary.LittleEndian.PutUint32(data[24:],8000);binary.LittleEndian.PutUint32(data[28:],16000);binary.LittleEndian.PutUint16(data[32:],2);binary.LittleEndian.PutUint16(data[34:],16);copy(data[36:],"data");binary.LittleEndian.PutUint32(data[40:],uint32(len(data)-44))
 if e:=os.WriteFile(path,data,0600);e!=nil{t.Fatal(e)}
 m:=&MPV{Device:"alsa/plughw:CARD=DoesNotExist,DEV=0"};defer m.Close()
 err:=m.Load(path,20)
 if err==nil{t.Fatal("missing configured device silently accepted")}
 if m.conn!=nil||m.cmd!=nil{t.Fatal("mpv process/connection not cleaned up after rejected device")}
}
// TestMPVPlaysRealLocalFormats verifies actual decoding (not just extension
// matching -- see internal/library.Scan) of every format Phase 2 requires:
// MP3, FLAC, OGG/Vorbis and M4A/AAC. Fixtures are generated on the fly with
// ffmpeg from a short, self-generated tone -- never checked into the repo.
func TestMPVPlaysRealLocalFormats(t *testing.T){
 if _,err:=exec.LookPath("mpv");err!=nil{t.Skip("mpv not installed; real decoding must be verified on target")}
 if _,err:=exec.LookPath("ffmpeg");err!=nil{t.Skip("ffmpeg not installed; format fixtures cannot be generated on this host")}
 dir:=t.TempDir()
 for _,format:=range []struct{ext,codec string}{{"mp3","libmp3lame"},{"flac","flac"},{"ogg","libvorbis"},{"m4a","aac"}}{
  t.Run(format.ext,func(t *testing.T){
   path:=filepath.Join(dir,"tone."+format.ext)
   cmd:=exec.Command("ffmpeg","-y","-hide_banner","-loglevel","error","-f","lavfi","-i","sine=frequency=440:duration=2","-c:a",format.codec,path)
   if out,err:=cmd.CombinedOutput();err!=nil{t.Fatalf("ffmpeg encode %s: %v: %s",format.ext,err,out)}
   m:=&MPV{NullOutput:true};defer m.Close()
   if e:=m.Load(path,20);e!=nil{t.Fatalf("mpv could not play a real .%s file: %v",format.ext,e)}
   time.Sleep(150*time.Millisecond)
   s,e:=m.Snapshot();if e!=nil{t.Fatal(e)}
   if s.Duration<1.5||s.Duration>2.5{t.Fatalf("unexpected duration for .%s: %+v",format.ext,s)}
  })
 }
}
// TestMPVAcceptsAutoDevice ensures the live device-list check does not
// reject the always-present "auto" selection.
func TestMPVAcceptsAutoDevice(t *testing.T){
 if _,err:=exec.LookPath("mpv");err!=nil{t.Skip("mpv not installed; device enumeration must be verified on target")}
 path:=filepath.Join(t.TempDir(),"tone.wav")
 data:=make([]byte,44+8000*2);copy(data,"RIFF");binary.LittleEndian.PutUint32(data[4:],uint32(len(data)-8));copy(data[8:],"WAVEfmt ");binary.LittleEndian.PutUint32(data[16:],16);binary.LittleEndian.PutUint16(data[20:],1);binary.LittleEndian.PutUint16(data[22:],1);binary.LittleEndian.PutUint32(data[24:],8000);binary.LittleEndian.PutUint32(data[28:],16000);binary.LittleEndian.PutUint16(data[32:],2);binary.LittleEndian.PutUint16(data[34:],16);copy(data[36:],"data");binary.LittleEndian.PutUint32(data[40:],uint32(len(data)-44))
 if e:=os.WriteFile(path,data,0600);e!=nil{t.Fatal(e)}
 m:=&MPV{NullOutput:true,Device:"auto"};defer m.Close()
 if e:=m.Load(path,20);e!=nil{t.Fatal(e)}
}
