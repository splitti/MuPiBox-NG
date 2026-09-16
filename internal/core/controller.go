// Package core is the single owner of queue and transport for every input module.
package core

import (
 "context"
 "errors"
 "fmt"
 "math"
 "sync"
 "time"

 "mupibox/internal/audio"
 "mupibox/internal/library"
)

type Command struct { Action string `json:"action"`; FolderID string `json:"folder_id,omitempty"`; Value float64 `json:"value,omitempty"` }
type Status struct {
 State string `json:"state"`
 Backend string `json:"backend"`
 FolderID string `json:"folder_id"`
 Folder string `json:"folder"`
 Cover string `json:"cover,omitempty"`
 Queue []library.Track `json:"queue"`
 Index int `json:"index"`
 Position float64 `json:"position"`
 Duration float64 `json:"duration"`
 Volume int `json:"volume"`
 MaxVolume int `json:"max_volume"`
 Error string `json:"error,omitempty"`
}
type Controller struct { mu sync.Mutex; backend audio.Backend; lib *library.Library; st Status }
func New(lib *library.Library,b audio.Backend,maxVolume int)(*Controller,error){
 if maxVolume<1||maxVolume>100{return nil,errors.New("max_volume must be 1..100")}
 return &Controller{backend:b,lib:lib,st:Status{State:"stopped",Backend:b.Name(),Queue:[]library.Track{},Index:-1,Volume:min(30,maxVolume),MaxVolume:maxVolume}},nil
}
func(c *Controller) Status()Status{c.mu.Lock();defer c.mu.Unlock();s:=c.st;s.Queue=append([]library.Track{},s.Queue...);return s}
func(c *Controller) Close()error{c.mu.Lock();defer c.mu.Unlock();return c.backend.Close()}
func(c *Controller) Run(ctx context.Context){t:=time.NewTicker(500*time.Millisecond);defer t.Stop();for{select{case<-ctx.Done():return;case<-t.C:c.Tick()}}}
func(c *Controller) fail(err error)error{_ = c.backend.Stop();c.st.State="error";c.st.Error=err.Error();return err}
func(c *Controller) load(index int)error{
 t:=c.st.Queue[index]
 if err:=c.lib.Validate(t.Path);err!=nil{return c.fail(err)}
 c.st.Index=index;c.st.Position=0;c.st.Duration=0
 if err:=c.backend.Load(t.Path,c.st.Volume);err!=nil{return c.fail(err)}
 c.st.State="playing";c.st.Error="";return nil
}
func(c *Controller) Tick(){
 c.mu.Lock();defer c.mu.Unlock()
 if c.st.State!="playing"&&c.st.State!="paused"{return}
 s,err:=c.backend.Snapshot();if err!=nil{_ = c.fail(err);return}
 c.st.Position=s.Position;c.st.Duration=s.Duration
 if s.Ended{
  if c.st.Index+1<len(c.st.Queue){_ = c.load(c.st.Index+1)}else{_ = c.backend.Stop();c.st.State="stopped"}
 }else if s.Paused{c.st.State="paused"}else{c.st.State="playing"}
}
func(c *Controller) Execute(cmd Command)error{
 c.mu.Lock();defer c.mu.Unlock()
 if math.IsNaN(cmd.Value)||math.IsInf(cmd.Value,0){return errors.New("invalid value")}
 switch cmd.Action{
 case "folder":
  f,ok:=c.lib.Folder(cmd.FolderID);if !ok||len(f.Tracks)==0{return errors.New("unknown or empty folder")}
  c.st.FolderID=f.ID;c.st.Folder=f.Name;c.st.Cover=f.Cover;c.st.Queue=append([]library.Track{},f.Tracks...)
  return c.load(0)
 case "volume","volume_delta":
  v:=cmd.Value;if cmd.Action=="volume_delta"{v+=float64(c.st.Volume)}
  v=math.Max(0,math.Min(float64(c.st.MaxVolume),v))
  if err:=c.backend.Volume(int(v));err!=nil{return c.fail(err)};c.st.Volume=int(v);return nil
 case "stop":
  if err:=c.backend.Stop();err!=nil{return c.fail(err)};c.st.State="stopped";c.st.Position=0;c.st.Error="";return nil
 case "play","pause","toggle","next","previous","seek":
  if len(c.st.Queue)==0{return errors.New("queue is empty")}
 default:return fmt.Errorf("unknown action %q",cmd.Action)
 }
 switch cmd.Action{
 case "next":if c.st.Index+1>=len(c.st.Queue){_ = c.backend.Stop();c.st.State="stopped";c.st.Position=0;return nil};return c.load(c.st.Index+1)
 case "previous":return c.load(max(0,c.st.Index-1))
 case "seek":
  if c.st.State!="playing"&&c.st.State!="paused"{return errors.New("player is not active")}
  if c.st.Duration<=0{return errors.New("duration not available yet")}
  pos:=math.Max(0,math.Min(c.st.Duration,cmd.Value));if err:=c.backend.Seek(pos);err!=nil{return c.fail(err)};c.st.Position=pos;return nil
 case "pause":if c.st.State!="playing"{return nil}
 }
 paused:=cmd.Action=="pause"||(cmd.Action=="toggle"&&c.st.State=="playing")
 if c.st.State=="stopped"||c.st.State=="error"{return c.load(max(0,c.st.Index))}
 if err:=c.backend.Pause(paused);err!=nil{return c.fail(err)}
 if paused{c.st.State="paused"}else{c.st.State="playing"};return nil
}
