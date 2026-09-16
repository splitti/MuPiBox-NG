package server

import (
 "encoding/json"
 "fmt"
 "io"
 "net/http"
 "net/url"
 "strings"

 "mupibox/internal/core"
 "mupibox/internal/library"
 "mupibox/internal/store"
 "mupibox/webui"
)

type InputConfig struct {
 Simulate bool `json:"simulate"`
 Buttons map[string]core.Command `json:"buttons"`
 RFID map[string]string `json:"rfid"` // UID -> folder ID (M1 only)
}
type Input struct { Module string `json:"module"`; ID string `json:"id"` }

type TTSConfig struct {
 Enabled bool `json:"enabled"`
 Language string `json:"language"`
 Provider string `json:"provider"`
}

type PowerConfig struct {
 IdleShutdownMinutes int `json:"idle_shutdown_minutes"`
}

type HomeConfig struct { Categories []HomeCategoryConfig `json:"categories"` }
type HomeCategoryConfig struct {
 ID string `json:"id"`
 Labels map[string]string `json:"labels"`
 Rows []HomeRowConfig `json:"rows,omitempty"`
}
type HomeRowConfig struct {
 ID string `json:"id"`
 Labels map[string]string `json:"labels"`
 Provider string `json:"provider"`
 SourceType string `json:"source_type,omitempty"`
 SourceRef string `json:"source_ref,omitempty"`
}

type Home struct { Categories []HomeCategory `json:"categories"` }
type HomeCategory struct {
 ID string `json:"id"`
 Labels map[string]string `json:"labels"`
 Rows []HomeRow `json:"rows"`
}
type HomeRow struct {
 ID string `json:"id"`
 Labels map[string]string `json:"labels"`
 Items []HomeItem `json:"items"`
}
type HomeItem struct {
 ID string `json:"id"`
 Kind string `json:"kind"`
 Title string `json:"title"`
 Subtitle string `json:"subtitle,omitempty"`
 Cover string `json:"cover,omitempty"`
 ResumePolicy string `json:"resume_policy,omitempty"`
 Command core.Command `json:"command"`
}

type API struct {
 Player *core.Controller
 Library *library.Library
 Inputs InputConfig
 HomeConfig HomeConfig
 TTS TTSConfig
 Power PowerConfig
 Store *store.Store
 Version string
}
func jsonResponse(w http.ResponseWriter,status int,v any){w.Header().Set("Content-Type","application/json");w.Header().Set("Cache-Control","no-store");w.WriteHeader(status);_ = json.NewEncoder(w).Encode(v)}
func problem(w http.ResponseWriter,status int,err error){jsonResponse(w,status,map[string]string{"error":err.Error()})}
func decode(w http.ResponseWriter,r *http.Request,v any)error{
 if !strings.HasPrefix(r.Header.Get("Content-Type"),"application/json"){return fmt.Errorf("Content-Type must be application/json")}
 r.Body=http.MaxBytesReader(w,r.Body,262144);d:=json.NewDecoder(r.Body);d.DisallowUnknownFields()
 if err:=d.Decode(v);err!=nil{return err};var extra any;if err:=d.Decode(&extra);err!=io.EOF{return fmt.Errorf("expected one JSON object")};return nil
}
func(a *API) localLibraryItems()[]HomeItem{
 items:=make([]HomeItem,0,len(a.Library.Folders))
 for _,f:=range a.Library.Folders{
  items=append(items,HomeItem{ID:f.ID,Kind:"local-folder",Title:f.Name,Subtitle:fmt.Sprintf("%d Titel",len(f.Tracks)),Cover:f.Cover,ResumePolicy:"position",Command:core.Command{Action:"folder",FolderID:f.ID}})
 }
 return items
}
func homeConfigFromNavigation(nav store.Navigation)HomeConfig{
 out:=HomeConfig{Categories:make([]HomeCategoryConfig,0,len(nav.Categories))}
 for _,c:=range nav.Categories{
  cat:=HomeCategoryConfig{ID:c.ID,Labels:c.Labels,Rows:make([]HomeRowConfig,0,len(c.Rows))}
  for _,r:=range c.Rows{cat.Rows=append(cat.Rows,HomeRowConfig{ID:r.ID,Labels:r.Labels,Provider:r.Provider,SourceType:r.SourceType,SourceRef:r.SourceRef})}
  out.Categories=append(out.Categories,cat)
 }
 return out
}
func(a *API) currentHomeConfig()HomeConfig{
 if a.Store!=nil{if nav,err:=a.Store.LoadNavigation();err==nil{return homeConfigFromNavigation(nav)}}
 return a.HomeConfig
}
func(a *API) Home()Home{
 cfg:=a.currentHomeConfig()
 if len(cfg.Categories)==0{
  cfg.Categories=[]HomeCategoryConfig{{ID:"music",Labels:map[string]string{"de":"Musik","en":"Music"},Rows:[]HomeRowConfig{{ID:"local-library",Labels:map[string]string{"de":"Lokale Musik","en":"Local music"},Provider:"local-library"}}}}
 }
 categories:=make([]HomeCategory,0,len(cfg.Categories))
 for _,category:=range cfg.Categories{
  out:=HomeCategory{ID:category.ID,Labels:category.Labels,Rows:[]HomeRow{}}
  for _,row:=range category.Rows{
   items:=[]HomeItem{}
   switch row.Provider{case "local-library":items=a.localLibraryItems()}
   out.Rows=append(out.Rows,HomeRow{ID:row.ID,Labels:row.Labels,Items:items})
  }
  categories=append(categories,out)
 }
 return Home{Categories:categories}
}
func(a *API) currentSettings()(store.BoxSettings,error){
 if a.Store!=nil{v,ok,err:=a.Store.LoadBoxSettings();if err!=nil{return store.BoxSettings{},err};if ok{return v,nil}}
 status:=a.Player.Status()
 return store.BoxSettings{Language:"de",AdminLanguage:"de",TTS:store.TTSSettings{Enabled:a.TTS.Enabled,Language:a.TTS.Language,Provider:a.TTS.Provider},Power:store.PowerSettings{IdleShutdownMinutes:a.Power.IdleShutdownMinutes},Audio:store.AudioSettings{StartupVolume:min(30,status.MaxVolume),MaxVolume:status.MaxVolume},Display:store.DisplaySettings{Brightness:100},Theme:"modern-dark"},nil
}
func(a *API) Handler()http.Handler{
 mux:=http.NewServeMux()
 mux.HandleFunc("GET /api/status",func(w http.ResponseWriter,r *http.Request){jsonResponse(w,200,a.Player.Status())})
 mux.HandleFunc("GET /api/library",func(w http.ResponseWriter,r *http.Request){jsonResponse(w,200,a.Library.Folders)})
 mux.HandleFunc("GET /api/home",func(w http.ResponseWriter,r *http.Request){jsonResponse(w,200,a.Home())})
 mux.HandleFunc("GET /api/info",func(w http.ResponseWriter,r *http.Request){
  settings,err:=a.currentSettings();if err!=nil{problem(w,500,err);return}
  jsonResponse(w,200,map[string]any{"version":a.Version,"simulation":a.Inputs.Simulate,"backend":a.Player.Status().Backend,"tts":settings.TTS,"power":settings.Power,"settings_persistent":a.Store!=nil})
 })
 mux.HandleFunc("GET /api/health",func(w http.ResponseWriter,r *http.Request){jsonResponse(w,200,map[string]string{"status":"ok","version":a.Version})})
 mux.HandleFunc("GET /api/admin/settings",func(w http.ResponseWriter,r *http.Request){if a.Store==nil{problem(w,503,fmt.Errorf("persistent store unavailable"));return};v,ok,err:=a.Store.LoadBoxSettings();if err!=nil{problem(w,500,err);return};if !ok{problem(w,404,fmt.Errorf("settings not initialized"));return};jsonResponse(w,200,v)})
 mux.HandleFunc("PUT /api/admin/settings",func(w http.ResponseWriter,r *http.Request){if a.Store==nil{problem(w,503,fmt.Errorf("persistent store unavailable"));return};var v store.BoxSettings;if err:=decode(w,r,&v);err!=nil{problem(w,400,err);return};if err:=a.Store.SaveBoxSettings(v);err!=nil{problem(w,400,err);return};a.TTS=TTSConfig{Enabled:v.TTS.Enabled,Language:v.TTS.Language,Provider:v.TTS.Provider};a.Power=PowerConfig{IdleShutdownMinutes:v.Power.IdleShutdownMinutes};jsonResponse(w,200,map[string]any{"settings":v,"restart_required":[]string{"audio.max_volume","audio.startup_volume"}})})
 mux.HandleFunc("GET /api/admin/navigation",func(w http.ResponseWriter,r *http.Request){if a.Store==nil{problem(w,503,fmt.Errorf("persistent store unavailable"));return};v,err:=a.Store.LoadNavigation();if err!=nil{problem(w,500,err);return};jsonResponse(w,200,v)})
 mux.HandleFunc("PUT /api/admin/navigation",func(w http.ResponseWriter,r *http.Request){if a.Store==nil{problem(w,503,fmt.Errorf("persistent store unavailable"));return};var v store.Navigation;if err:=decode(w,r,&v);err!=nil{problem(w,400,err);return};if err:=a.Store.SaveNavigation(v);err!=nil{problem(w,400,err);return};jsonResponse(w,200,v)})
 mux.HandleFunc("GET /api/cover/{id}",func(w http.ResponseWriter,r *http.Request){
  f,ok:=a.Library.Folder(r.PathValue("id"));if !ok||f.CoverPath==""{http.NotFound(w,r);return}
  if err:=a.Library.Validate(f.CoverPath);err!=nil{http.NotFound(w,r);return};http.ServeFile(w,r,f.CoverPath)
 })
 mux.HandleFunc("POST /api/command",func(w http.ResponseWriter,r *http.Request){var cmd core.Command;if err:=decode(w,r,&cmd);err!=nil{problem(w,400,err);return};a.execute(w,cmd)})
 mux.HandleFunc("POST /api/input",func(w http.ResponseWriter,r *http.Request){
  if !a.Inputs.Simulate{http.NotFound(w,r);return}
  var in Input;if err:=decode(w,r,&in);err!=nil{problem(w,400,err);return}
  cmd,err:=a.ResolveInput(in);if err!=nil{problem(w,400,err);return};a.execute(w,cmd)
 })
 mux.HandleFunc("GET /api/",func(w http.ResponseWriter,r *http.Request){http.NotFound(w,r)})
 mux.Handle("GET /",webui.Handler())
 return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){
  w.Header().Set("X-Content-Type-Options","nosniff")
  w.Header().Set("Content-Security-Policy","default-src 'self'; img-src 'self' data:; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
  if r.Method!="GET"&&r.Method!="HEAD"{
   if r.Header.Get("Sec-Fetch-Site")=="cross-site"{problem(w,403,fmt.Errorf("cross-site request denied"));return}
   if origin:=r.Header.Get("Origin");origin!=""{u,err:=url.Parse(origin);scheme:="http";if r.TLS!=nil{scheme="https"};if err!=nil||u.Host!=r.Host||u.Scheme!=scheme{problem(w,403,fmt.Errorf("origin denied"));return}}
  }
  mux.ServeHTTP(w,r)
 })
}
func(a *API) execute(w http.ResponseWriter,cmd core.Command){if err:=a.Player.Execute(cmd);err!=nil{problem(w,400,err);return};jsonResponse(w,200,a.Player.Status())}
func(a *API) ResolveInput(in Input)(core.Command,error){
 switch in.Module{
 case "button":if cmd,ok:=a.Inputs.Buttons[in.ID];ok{return cmd,nil}
 case "rfid":if folder,ok:=a.Inputs.RFID[in.ID];ok{return core.Command{Action:"folder",FolderID:folder},nil}
 };return core.Command{},fmt.Errorf("unmapped input")
}
