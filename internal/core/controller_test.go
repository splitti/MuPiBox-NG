package core

import (
 "errors"
 "os"
 "path/filepath"
 "sync"
 "testing"
 "mupibox/internal/audio"
 "mupibox/internal/library"
)
type fakeAudio struct{audio.Simulated; snapshot audio.Snapshot; loadError bool; loaded int; volume int}
func(f *fakeAudio) Load(p string,v int)error{if f.loadError{return errors.New("decoder unavailable")};f.loaded++;f.volume=v;return nil}
func(f *fakeAudio) Snapshot()(audio.Snapshot,error){return f.snapshot,nil}
func(f *fakeAudio) Volume(v int)error{f.volume=v;return nil}
func setup(t *testing.T)(*Controller,*fakeAudio,string){t.Helper();dir:=t.TempDir();for _,n:=range []string{"1.wav","2.wav"}{if e:=os.WriteFile(filepath.Join(dir,n),nil,0600);e!=nil{t.Fatal(e)}};l,e:=library.Scan(dir);if e!=nil{t.Fatal(e)};b:=&fakeAudio{};c,e:=New(l,b,45);if e!=nil{t.Fatal(e)};t.Cleanup(func(){c.Close()});return c,b,l.Folders[0].ID}
func TestQueueCommandsAndEOF(t *testing.T){
 c,b,id:=setup(t)
 if e:=c.Execute(Command{Action:"play"});e==nil{t.Fatal("empty queue played")}
 if e:=c.Execute(Command{Action:"folder",FolderID:id});e!=nil{t.Fatal(e)}
 if e:=c.Execute(Command{Action:"volume",Value:100});e!=nil{t.Fatal(e)};if c.Status().Volume!=45||b.volume!=45{t.Fatal("volume cap bypassed")}
 b.snapshot=audio.Snapshot{Position:10,Duration:30};c.Tick()
 c.Execute(Command{Action:"pause"});if c.Status().State!="paused"{t.Fatal("not paused")}
 c.Execute(Command{Action:"toggle"});if c.Status().State!="playing"{t.Fatal("not resumed")}
 b.snapshot.Ended=true;c.Tick();if c.Status().Index!=1||b.loaded!=2{t.Fatal("no auto-next")}
 c.Tick();if c.Status().State!="stopped"{t.Fatal("end of queue not stopped")}
 copy:=c.Status();copy.Queue[0].Title="changed";if c.Status().Queue[0].Title=="changed"{t.Fatal("mutable snapshot")}
}
func TestDecoderFailureDoesNotPretendPlayback(t *testing.T){c,b,id:=setup(t);b.loadError=true;if c.Execute(Command{Action:"folder",FolderID:id})==nil{t.Fatal("expected error")};if c.Status().State!="error"||c.Status().Error==""{t.Fatal("false playback state")}}
func TestConcurrentInputs(t *testing.T){c,_,id:=setup(t);c.Execute(Command{Action:"folder",FolderID:id});var wg sync.WaitGroup;for i:=0;i<8;i++{wg.Add(1);go func(){defer wg.Done();for j:=0;j<30;j++{c.Execute(Command{Action:"volume_delta",Value:1});_ = c.Status()}}()};wg.Wait();if c.Status().Volume!=45{t.Fatal("cap lost under concurrency")}}
