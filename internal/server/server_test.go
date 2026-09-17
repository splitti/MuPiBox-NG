package server

import (
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "os"
 "path/filepath"
 "strings"
 "testing"
 "mupibox/internal/audio"
 "mupibox/internal/core"
 "mupibox/internal/library"
 "mupibox/internal/store"
)
func TestAllInputsControlOnePlayer(t *testing.T){
 dir:=t.TempDir();os.WriteFile(filepath.Join(dir,"track.wav"),nil,0600);l,_:=library.Scan(dir);p,_:=core.New(l,&audio.Simulated{},50);defer p.Close()
 a:=&API{Player:p,Library:l,Inputs:InputConfig{Simulate:true,Buttons:map[string]core.Command{"pause":{Action:"pause"}},RFID:map[string]string{"card":l.Folders[0].ID}}};h:=a.Handler()
 req:=func(path,body,origin string)int{r:=httptest.NewRequest(http.MethodPost,path,strings.NewReader(body));r.Header.Set("Content-Type","application/json");if origin!=""{r.Header.Set("Origin",origin)};w:=httptest.NewRecorder();h.ServeHTTP(w,r);return w.Code}
 if req("/api/input",`{"module":"rfid","id":"card"}`,"")!=200{t.Fatal("RFID failed")}
 if req("/api/input",`{"module":"button","id":"pause"}`,"")!=200||p.Status().State!="paused"{t.Fatal("button did not pause RFID playback")}
 if req("/api/command",`{"action":"play"}`,"")!=200||p.Status().State!="playing"{t.Fatal("browser did not resume")}
 if req("/api/command",`{"action":"pause"}`,"https://other.example")!=403{t.Fatal("cross-origin accepted")}
 if req("/api/command",`{"action":"pause","extra":1}`,"")!=400{t.Fatal("unknown field accepted")}
 if req("/api/command",`{"action":"pause"} {}`,"")!=400{t.Fatal("trailing data accepted")}
 a.Inputs.Simulate=false;if req("/api/input",`{"module":"rfid","id":"card"}`,"")!=404{t.Fatal("simulation exposed")}
 w:=httptest.NewRecorder();h.ServeHTTP(w,httptest.NewRequest("GET","/api/status",nil));var s core.Status;if err:=json.Unmarshal(w.Body.Bytes(),&s);err!=nil||s.State!="playing"{t.Fatal("shared status failed",err)}
 if strings.Contains(w.Body.String(),dir){t.Fatal("filesystem path leaked")}
}

func TestHomeConfigIsDataDrivenAndLocalized(t *testing.T){
 dir:=t.TempDir();os.WriteFile(filepath.Join(dir,"track.wav"),nil,0600);l,_:=library.Scan(dir);p,_:=core.New(l,&audio.Simulated{},50);defer p.Close()
 a:=&API{Player:p,Library:l,HomeConfig:HomeConfig{Categories:[]HomeCategoryConfig{
  {ID:"books",Labels:map[string]string{"de":"Hörbücher","en":"Audiobooks"}},
  {ID:"music",Labels:map[string]string{"de":"Musik","en":"Music"},Rows:[]HomeRowConfig{{ID:"local",Labels:map[string]string{"de":"Lokal","en":"Local"},Provider:"local-library"}}},
 }}}
 w:=httptest.NewRecorder();a.Handler().ServeHTTP(w,httptest.NewRequest("GET","/api/home",nil));if w.Code!=200{t.Fatalf("home status=%d",w.Code)}
 var home Home;if err:=json.Unmarshal(w.Body.Bytes(),&home);err!=nil{t.Fatal(err)}
 if len(home.Categories)!=2||home.Categories[0].Labels["en"]!="Audiobooks"{t.Fatalf("unexpected categories: %#v",home.Categories)}
 if len(home.Categories[1].Rows)!=1||len(home.Categories[1].Rows[0].Items)!=1{t.Fatalf("local provider not rendered: %#v",home.Categories[1])}
 item:=home.Categories[1].Rows[0].Items[0]
 if item.Command.FolderID!=l.Folders[0].ID{t.Fatal("media command does not target scanned folder")}
 if item.Provider!="local-library"||!item.OfflineAvailable{t.Fatalf("local item must remain available offline: %#v",item)}
}

func TestInfoExposesGlobalBoxSettings(t *testing.T){
 dir:=t.TempDir();os.WriteFile(filepath.Join(dir,"track.wav"),nil,0600);l,_:=library.Scan(dir);p,_:=core.New(l,&audio.Simulated{},50);defer p.Close()
 a:=&API{Player:p,Library:l,TTS:TTSConfig{Enabled:true,Language:"de",Provider:"browser-dev"},Power:PowerConfig{IdleShutdownMinutes:30}}
 w:=httptest.NewRecorder();a.Handler().ServeHTTP(w,httptest.NewRequest("GET","/api/info",nil));if w.Code!=200{t.Fatalf("info status=%d",w.Code)}
 var info struct{TTS TTSConfig `json:"tts"`; Power PowerConfig `json:"power"`};if err:=json.Unmarshal(w.Body.Bytes(),&info);err!=nil{t.Fatal(err)}
 if !info.TTS.Enabled||info.TTS.Language!="de"||info.Power.IdleShutdownMinutes!=30{t.Fatalf("unexpected info: %#v",info)}
}


func TestAdminPersistsSettingsAndNavigation(t *testing.T){
 dir:=t.TempDir();booksDir:=filepath.Join(dir,"books");os.Mkdir(booksDir,0700);os.WriteFile(filepath.Join(booksDir,"track.wav"),nil,0600);l,_:=library.Scan(dir);p,_:=core.New(l,&audio.Simulated{},60);defer p.Close()
 db,err:=store.Open(":memory:");if err!=nil{t.Fatal(err)};defer db.Close()
 defaults:=store.BoxSettings{Language:"de",AdminLanguage:"de",TTS:store.TTSSettings{Language:"de"},Audio:store.AudioSettings{StartupVolume:30,MaxVolume:60},Display:store.DisplaySettings{Brightness:100},Theme:"modern-dark"}
 if _,err=db.EnsureBoxSettings(defaults);err!=nil{t.Fatal(err)}
 if err=db.EnsureNavigation(store.Navigation{Categories:[]store.Category{{ID:"music",Labels:map[string]string{"de":"Musik"},Rows:[]store.Row{{ID:"local",Labels:map[string]string{"de":"Lokal"},Provider:"local-library"}}}}});err!=nil{t.Fatal(err)}
 a:=&API{Player:p,Library:l,Store:db};h:=a.Handler()
 put:=func(path,body string)*httptest.ResponseRecorder{r:=httptest.NewRequest(http.MethodPut,path,strings.NewReader(body));r.Header.Set("Content-Type","application/json");w:=httptest.NewRecorder();h.ServeHTTP(w,r);return w}
 w:=put("/api/admin/settings",`{"language":"de","admin_language":"en","tts":{"enabled":false,"language":"de","provider":""},"power":{"idle_shutdown_minutes":20},"audio":{"startup_volume":20,"max_volume":50,"start_sound_enabled":true,"shutdown_sound_enabled":true},"display":{"idle_off_minutes":5,"brightness":70},"theme":"arcade-8bit"}`)
 if w.Code!=200{t.Fatalf("settings status=%d body=%s",w.Code,w.Body.String())}
 saved,ok,err:=db.LoadBoxSettings();if err!=nil||!ok||saved.Theme!="arcade-8bit"||saved.AdminLanguage!="en"||saved.Power.IdleShutdownMinutes!=20{t.Fatalf("settings not persisted: %#v %v",saved,err)}
 w=put("/api/admin/navigation",`{"categories":[{"id":"books","labels":{"de":"Hörbücher","en":"Audiobooks"},"rows":[{"id":"local-books","labels":{"de":"Lokal"},"provider":"local-library","source_type":"path","source_ref":"books"}]}]}`)
 if w.Code!=200{t.Fatalf("navigation status=%d body=%s",w.Code,w.Body.String())}
 savedNav,err:=db.LoadNavigation();if err!=nil||savedNav.Categories[0].Rows[0].SourceType!="path"||savedNav.Categories[0].Rows[0].SourceRef!="books"{t.Fatalf("media source not persisted: %#v %v",savedNav,err)}
 home:=a.Home();if len(home.Categories)!=1||home.Categories[0].ID!="books"||len(home.Categories[0].Rows[0].Items)!=1{t.Fatalf("persisted navigation not rendered: %#v",home)}
}


func TestPlayerUsesClockHoldForAdminWithoutSettingsDialog(t *testing.T){
 dir:=t.TempDir();os.WriteFile(filepath.Join(dir,"track.wav"),nil,0600);l,_:=library.Scan(dir);p,_:=core.New(l,&audio.Simulated{},50);defer p.Close()
 h:=(&API{Player:p,Library:l}).Handler()
 get:=func(path string)string{w:=httptest.NewRecorder();h.ServeHTTP(w,httptest.NewRequest(http.MethodGet,path,nil));if w.Code!=200{t.Fatalf("%s status=%d",path,w.Code)};return w.Body.String()}
 page:=get("/")
 if !strings.Contains(page,`id="clock"`)||strings.Contains(page,`id="settings"`)||strings.Contains(page,"<dialog"){t.Fatalf("unexpected player admin controls: %s",page)}
 script:=get("/app.js")
 if !strings.Contains(script,"5000")||!strings.Contains(script,"/admin/")||strings.Contains(script,"/api/input"){t.Fatal("player must open admin only through five-second clock hold")}
 admin:=get("/admin/")
 if strings.Contains(admin,"Name DE")||strings.Contains(admin,"Name EN")||!strings.Contains(admin,`id="content-language"`){t.Fatal("admin must edit one localized name at a time")}
 _=get("/admin/locales/de.json");_=get("/admin/locales/en.json")
 logo:=get("/mupibox-logo.svg");if !strings.Contains(logo,"data:image/jpeg;base64,/9j/")||!strings.Contains(logo,"</svg>"){t.Fatal("embedded MuPiBox logo is invalid")}
}

func TestAdminListsDirectoriesRescansAndRestartsUI(t *testing.T){
 root:=t.TempDir();stories:=filepath.Join(root,"Stories");if err:=os.Mkdir(stories,0700);err!=nil{t.Fatal(err)}
 if err:=os.WriteFile(filepath.Join(stories,"one.mp3"),nil,0600);err!=nil{t.Fatal(err)}
 lib,err:=library.Scan(root);if err!=nil{t.Fatal(err)}
 player,err:=core.New(lib,&audio.Simulated{},50);if err!=nil{t.Fatal(err)};defer player.Close()
 api:=&API{Player:player,Library:lib};handler:=api.Handler()
 request:=func(method,path string)*httptest.ResponseRecorder{response:=httptest.NewRecorder();handler.ServeHTTP(response,httptest.NewRequest(method,path,nil));return response}
 directories:=request(http.MethodGet,"/api/admin/local-directories");if directories.Code!=200||!strings.Contains(directories.Body.String(),`"path":"Stories"`){t.Fatalf("directories: %d %s",directories.Code,directories.Body.String())}
 more:=filepath.Join(root,"Music");if err=os.Mkdir(more,0700);err!=nil{t.Fatal(err)};if err=os.WriteFile(filepath.Join(more,"two.mp3"),nil,0600);err!=nil{t.Fatal(err)}
 rescan:=request(http.MethodPost,"/api/admin/library/rescan");if rescan.Code!=200||!strings.Contains(rescan.Body.String(),`"folders":2`){t.Fatalf("rescan: %d %s",rescan.Code,rescan.Body.String())}
 before:=request(http.MethodGet,"/api/ui-state");restart:=request(http.MethodPost,"/api/admin/ui/restart");after:=request(http.MethodGet,"/api/ui-state")
 if before.Code!=200||restart.Code!=202||after.Code!=200||!strings.Contains(after.Body.String(),`"restart_generation":1`){t.Fatalf("restart state: before=%s restart=%s after=%s",before.Body.String(),restart.Body.String(),after.Body.String())}
}

func TestResumeListSourceReturnsLatestIncompleteLocalMedia(t *testing.T){
 dir:=t.TempDir();folderDir:=filepath.Join(dir,"Story");if err:=os.Mkdir(folderDir,0700);err!=nil{t.Fatal(err)}
 for _,name:=range []string{"01.wav","02.wav"}{if err:=os.WriteFile(filepath.Join(folderDir,name),nil,0600);err!=nil{t.Fatal(err)}}
 lib,err:=library.Scan(dir);if err!=nil{t.Fatal(err)};player,err:=core.New(lib,&audio.Simulated{},60);if err!=nil{t.Fatal(err)};defer player.Close()
 db,err:=store.Open(":memory:");if err!=nil{t.Fatal(err)};defer db.Close()
 folder:=lib.Folders[0];track:=folder.Tracks[1]
 if err=db.PutProgress("local","",track.ID,120000,3600000,folder.ID,1,false);err!=nil{t.Fatal(err)}
 if err=db.SaveNavigation(store.Navigation{Categories:[]store.Category{{ID:"continue",Labels:map[string]string{"de":"Zuletzt gehört"},Rows:[]store.Row{{ID:"recent",Labels:map[string]string{"de":"Fortsetzen"},Provider:"resume-list",SourceType:"limit",SourceRef:"10"}}}}});err!=nil{t.Fatal(err)}
 api:=&API{Player:player,Library:lib,Store:db}
 home:=api.Home();if len(home.Categories)!=1||len(home.Categories[0].Rows)!=1||len(home.Categories[0].Rows[0].Items)!=1{t.Fatalf("resume item missing: %#v",home)}
 item:=home.Categories[0].Rows[0].Items[0]
 if item.Kind!="resume"||item.Command.Action!="resume"||item.Command.FolderID!=folder.ID||item.Command.ItemIndex!=1||!item.OfflineAvailable{t.Fatalf("unexpected resume item: %#v",item)}
 if err=player.Execute(item.Command);err!=nil||player.Status().Index!=1{t.Fatalf("resume command failed: %#v %v",player.Status(),err)}
}
