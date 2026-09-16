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
type fakeAudio struct{audio.Simulated; snapshot audio.Snapshot; loadError bool; loaded int; volume int; seek float64}
func(f *fakeAudio) Load(p string,v int)error{if f.loadError{return errors.New("decoder unavailable")};f.loaded++;f.volume=v;return nil}
func(f *fakeAudio) Snapshot()(audio.Snapshot,error){return f.snapshot,nil}
func(f *fakeAudio) Volume(v int)error{f.volume=v;return nil}
func(f *fakeAudio) Seek(v float64)error{f.seek=v;return nil}
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

type fakeProgress struct{position,duration int64;found,completed bool;puts int;provider,media string}
func(f *fakeProgress)PutProgress(provider,accountID,mediaID string,positionMS,durationMS int64,contextID string,itemIndex int,completed bool)error{f.puts++;f.provider=provider;f.media=mediaID;f.position=positionMS;f.duration=durationMS;f.completed=completed;return nil}
func(f *fakeProgress)GetProgress(provider,accountID,mediaID string)(int64,int64,bool,bool,error){f.provider=provider;f.media=mediaID;return f.position,f.duration,f.completed,f.found,nil}

func TestPersistentResumeForLongSingleTrack(t *testing.T){
 c,b,id:=setup(t);repo:=&fakeProgress{position:6*60*60*1000,duration:23*60*60*1000,found:true};c.SetProgressRepository(repo)
 if err:=c.Execute(Command{Action:"folder",FolderID:id});err!=nil{t.Fatal(err)}
 if b.seek!=6*60*60||c.Status().Position!=6*60*60{t.Fatalf("did not resume long item: seek=%v status=%v",b.seek,c.Status().Position)}
 b.snapshot=audio.Snapshot{Position:6*60*60+10,Duration:23*60*60};c.Tick()
 if err:=c.Execute(Command{Action:"pause"});err!=nil{t.Fatal(err)}
 if repo.puts==0||repo.provider!="local"||repo.media==""{t.Fatalf("progress not stored: %#v",repo)}
}
