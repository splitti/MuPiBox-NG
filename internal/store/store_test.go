package store

import "testing"

func TestMigrationsSettingsNavigationAndProgress(t *testing.T){
 s,err:=Open(":memory:");if err!=nil{t.Fatal(err)};defer s.Close()
 defaults:=BoxSettings{Language:"de",AdminLanguage:"de",TTS:TTSSettings{Enabled:true,Language:"de",Provider:"offline"},Power:PowerSettings{IdleShutdownMinutes:30},Audio:AudioSettings{StartupVolume:25,MaxVolume:60},Display:DisplaySettings{IdleOffMinutes:5,Brightness:80},Theme:"modern-dark"}
 got,err:=s.EnsureBoxSettings(defaults);if err!=nil||got.Audio.MaxVolume!=60{t.Fatalf("settings: %#v %v",got,err)}
 got.Audio.MaxVolume=70;if err=s.SaveBoxSettings(got);err!=nil{t.Fatal(err)}
 got,ok,err:=s.LoadBoxSettings();if err!=nil||!ok||got.Audio.MaxVolume!=70{t.Fatalf("saved settings: %#v %v",got,err)}

 nav:=Navigation{Categories:[]Category{{ID:"books",Labels:map[string]string{"de":"Hörbücher","en":"Audiobooks"},Rows:[]Row{{ID:"local-books",Labels:map[string]string{"de":"Lokal"},Provider:"spotify",SourceType:"artist",SourceRef:"spotify:artist:example"}}}}}
 if err=s.EnsureNavigation(nav);err!=nil{t.Fatal(err)}
 loaded,err:=s.LoadNavigation();if err!=nil||len(loaded.Categories)!=1||len(loaded.Categories[0].Rows)!=1||loaded.Categories[0].Rows[0].SourceType!="artist"||loaded.Categories[0].Rows[0].SourceRef!="spotify:artist:example"{t.Fatalf("navigation: %#v %v",loaded,err)}

 p:=Progress{Provider:"spotify",AccountID:"box-account",MediaID:"episode-1",PositionMS:21600000,DurationMS:82800000}
 if err=s.SaveProgress(p);err!=nil{t.Fatal(err)}
 pg,ok,err:=s.LoadProgress("spotify","box-account","episode-1");if err!=nil||!ok||pg.PositionMS!=21600000{t.Fatalf("progress: %#v %v",pg,err)}
 if err=s.SaveProgress(Progress{Provider:"radio",MediaID:"live",PositionMS:1000});err!=nil{t.Fatal(err)}
 if _,ok,err=s.LoadProgress("radio","","live");err!=nil||ok{t.Fatal("radio progress must not be stored")}
}

func TestValidation(t *testing.T){
 s,err:=Open(":memory:");if err!=nil{t.Fatal(err)};defer s.Close()
 bad:=BoxSettings{Language:"de",Audio:AudioSettings{StartupVolume:70,MaxVolume:60},Display:DisplaySettings{Brightness:80},Theme:"modern-dark"}
 if err=s.SaveBoxSettings(bad);err==nil{t.Fatal("invalid settings accepted")}
 if err=s.SaveNavigation(Navigation{Categories:[]Category{{ID:"bad id",Labels:map[string]string{"de":"Bad"}}}});err==nil{t.Fatal("invalid navigation accepted")}
 if err=s.SaveNavigation(Navigation{Categories:[]Category{{ID:"books",Labels:map[string]string{"de":"Bücher"},Rows:[]Row{{ID:"escape",Labels:map[string]string{"de":"Escape"},Provider:"local-library",SourceType:"path",SourceRef:"../private"}}}}});err==nil{t.Fatal("escaping local path accepted")}
}
