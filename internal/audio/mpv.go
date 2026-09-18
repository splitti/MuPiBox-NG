package audio

import (
 "encoding/json"
 "errors"
 "fmt"
 "net"
 "os"
 "os/exec"
 "path/filepath"
 "time"
)

// MPV uses a private Unix socket. No network IPC or shell interpolation.
// A fresh process per track isolates late events from the preceding track.
// Device is the mpv --audio-device value (e.g. "alsa/hw:CARD=sndrpimax98357a,DEV=0"); empty means mpv picks its own default.
type MPV struct { NullOutput bool; Device string; cmd *exec.Cmd; conn net.Conn; decoder *json.Decoder; dir string; seq int }
type reply struct { ID int `json:"request_id"`; Error string `json:"error"`; Event string `json:"event"`; Reason string `json:"reason"`; Data json.RawMessage `json:"data"` }
func (*MPV) Name()string{return "mpv"}
func (m *MPV) Load(path string,volume int)(err error){
 if err=m.Stop();err!=nil{return err}
 m.dir,err=os.MkdirTemp("","mupibox-mpv-");if err!=nil{return err}
 defer func(){if err!=nil{_ = m.Stop()}}()
 socket:=filepath.Join(m.dir,"ipc")
 args:=[]string{"--no-config","--no-video","--no-terminal","--input-default-bindings=no","--input-vo-keyboard=no","--idle=yes","--keep-open=yes","--pause=yes","--volume="+fmt.Sprint(volume),"--input-ipc-server="+socket}
 if m.NullOutput{args=append(args,"--ao=null")}
 if m.Device!=""{args=append(args,"--audio-device="+m.Device)}
 m.cmd=exec.Command("mpv",args...)
 m.cmd.Stderr=os.Stderr
 if err=m.cmd.Start();err!=nil{m.cmd=nil;return fmt.Errorf("start mpv: %w",err)}
 deadline:=time.Now().Add(5*time.Second)
 for time.Now().Before(deadline){
  m.conn,err=net.DialTimeout("unix",socket,100*time.Millisecond)
  if err==nil{break};time.Sleep(20*time.Millisecond)
 }
 if err!=nil{return fmt.Errorf("mpv IPC: %w",err)}
 m.decoder=json.NewDecoder(m.conn)
 // Send load without discarding file-loaded events that can precede its reply.
 m.seq++;request:=m.seq
 _=m.conn.SetDeadline(time.Now().Add(10*time.Second))
 if err=json.NewEncoder(m.conn).Encode(map[string]any{"command":[]any{"loadfile",path,"replace"},"request_id":request});err!=nil{return err}
 loaded,accepted:=false,false
 for !loaded||!accepted{
  var r reply;if err=m.decoder.Decode(&r);err!=nil{return fmt.Errorf("load audio: %w",err)}
  if r.Event=="end-file"&&r.Reason=="error"{return fmt.Errorf("mpv could not decode audio: %s",r.Error)}
  if r.Event=="file-loaded"{loaded=true}
  if r.ID==request{if r.Error!="success"{return errors.New(r.Error)};accepted=true}
 }
 return m.Pause(false)
}
func (m *MPV) call(out any,args ...any)error{
 if m.conn==nil{return errors.New("mpv is not running")}
 _=m.conn.SetDeadline(time.Now().Add(2*time.Second));m.seq++
 if err:=json.NewEncoder(m.conn).Encode(map[string]any{"command":args,"request_id":m.seq});err!=nil{return err}
 for{var r reply;if err:=m.decoder.Decode(&r);err!=nil{return err};if r.ID!=m.seq{continue};if r.Error!="success"{return errors.New(r.Error)};if out!=nil{return json.Unmarshal(r.Data,out)};return nil}
}
func(m *MPV) Pause(v bool)error{return m.call(nil,"set_property","pause",v)}
func(m *MPV) Volume(v int)error{if m.conn==nil{return nil};return m.call(nil,"set_property","volume",v)}
func(m *MPV) Seek(v float64)error{return m.call(nil,"seek",v,"absolute+exact")}
func(m *MPV) Snapshot()(Snapshot,error){
 var s Snapshot
 for _,q:=range []struct{name string;out any}{{"time-pos",&s.Position},{"duration",&s.Duration},{"pause",&s.Paused},{"eof-reached",&s.Ended}}{
  if err:=m.call(q.out,"get_property",q.name);err!=nil{return s,fmt.Errorf("mpv %s: %w",q.name,err)}
 };return s,nil
}
func(m *MPV) Stop()error{
 if m.conn!=nil{_ = m.conn.Close();m.conn=nil}
 if m.cmd!=nil{if m.cmd.Process!=nil{_ = m.cmd.Process.Kill();_ = m.cmd.Wait()};m.cmd=nil}
 if m.dir!=""{err:=os.RemoveAll(m.dir);m.dir="";return err};return nil
}
func(m *MPV) Close()error{return m.Stop()}
