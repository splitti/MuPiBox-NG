package server

import (
 "encoding/json"
 "fmt"
 "io"
 "net/http"
 "net/url"
 pathpkg "path"
 "strconv"
 "strings"
 "sync"
 "sync/atomic"

 "mupibox/internal/connectivity"
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
 Provider string `json:"provider,omitempty"`
 OfflineAvailable bool `json:"offline_available"`
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
 Connectivity *connectivity.Manager
 System func() SystemStatus
 Version string
 libraryMu sync.RWMutex
 uiRestartGeneration atomic.Uint64
 adminSessionInit sync.Once
 adminSessions *adminSessionStore
}
func jsonResponse(w http.ResponseWriter,status int,v any){w.Header().Set("Content-Type","application/json");w.Header().Set("Cache-Control","no-store");w.WriteHeader(status);_ = json.NewEncoder(w).Encode(v)}
func problem(w http.ResponseWriter,status int,err error){jsonResponse(w,status,map[string]string{"error":err.Error()})}
func decode(w http.ResponseWriter,r *http.Request,v any)error{
 if !strings.HasPrefix(r.Header.Get("Content-Type"),"application/json"){return fmt.Errorf("Content-Type must be application/json")}
 r.Body=http.MaxBytesReader(w,r.Body,262144);d:=json.NewDecoder(r.Body);d.DisallowUnknownFields()
 if err:=d.Decode(v);err!=nil{return err};var extra any;if err:=d.Decode(&extra);err!=io.EOF{return fmt.Errorf("expected one JSON object")};return nil
}
func(a *API) librarySnapshot()*library.Library{a.libraryMu.RLock();defer a.libraryMu.RUnlock();return a.Library}
func(a *API) rescanLibrary()error{
 current:=a.librarySnapshot();if current==nil{return fmt.Errorf("library unavailable")}
 next,err:=library.Scan(current.Root);if err!=nil{return err}
 a.libraryMu.Lock();a.Library=next;a.libraryMu.Unlock()
 if a.Player!=nil{a.Player.SetLibrary(next)}
 return nil
}
func(a *API) localDirectories()([]library.Directory,error){lib:=a.librarySnapshot();if lib==nil{return nil,fmt.Errorf("library unavailable")};return library.Directories(lib.Root)}
func(a *API) localLibraryItems(row HomeRowConfig)[]HomeItem{
 lib:=a.librarySnapshot();if lib==nil{return []HomeItem{}}
 selected:=""
 if row.SourceType=="path"{selected=pathpkg.Clean(strings.Trim(strings.ReplaceAll(row.SourceRef,"\\","/"),"/"));if selected=="."{selected=""}}
 items:=make([]HomeItem,0,len(lib.Folders))
 for _,f:=range lib.Folders{
  if selected!=""&&f.Relative!=selected&&!strings.HasPrefix(f.Relative,selected+"/"){continue}
  items=append(items,HomeItem{ID:f.ID,Kind:"local-folder",Title:f.Name,Subtitle:fmt.Sprintf("%d Titel",len(f.Tracks)),Cover:f.Cover,ResumePolicy:"position",Provider:"local-library",OfflineAvailable:true,Command:core.Command{Action:"folder",FolderID:f.ID}})
 }
 return items
}
func(a *API)resumeItems(row HomeRowConfig)[]HomeItem{
 if a.Store==nil{return []HomeItem{}}
 limit,err:=strconv.Atoi(strings.TrimSpace(row.SourceRef));if err!=nil||limit<1{limit=10};if limit>100{limit=100}
 recent,err:=a.Store.ListRecentProgress(min(1000,limit*10));if err!=nil{return []HomeItem{}}
 lib:=a.librarySnapshot();if lib==nil{return []HomeItem{}}
 items:=make([]HomeItem,0,limit);seen:=map[string]bool{}
 for _,progress:=range recent{
  if progress.Provider!="local"||progress.ContextID==""||seen[progress.ContextID]{continue}
  folder,ok:=lib.Folder(progress.ContextID);if !ok||len(folder.Tracks)==0{continue}
  index:=progress.ItemIndex
  if index<0||index>=len(folder.Tracks)||folder.Tracks[index].ID!=progress.MediaID{
   index=-1;for i,track:=range folder.Tracks{if track.ID==progress.MediaID{index=i;break}}
  }
  if index<0{continue}
  track:=folder.Tracks[index];subtitle:=track.Title
  if progress.DurationMS>0{percent:=int(progress.PositionMS*100/progress.DurationMS);if percent>99{percent=99};subtitle=fmt.Sprintf("%s · %d%%",track.Title,percent)}
  seen[progress.ContextID]=true
  items=append(items,HomeItem{ID:"resume-"+progress.Provider+"-"+progress.MediaID,Kind:"resume",Title:folder.Name,Subtitle:subtitle,Cover:folder.Cover,ResumePolicy:"position",Provider:progress.Provider,OfflineAvailable:true,Command:core.Command{Action:"resume",FolderID:folder.ID,ItemIndex:index}})
  if len(items)>=limit{break}
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
   switch row.Provider{case "local-library":items=a.localLibraryItems(row);case "resume-list":items=a.resumeItems(row)}
   out.Rows=append(out.Rows,HomeRow{ID:row.ID,Labels:row.Labels,Items:items})
  }
  categories=append(categories,out)
 }
 return Home{Categories:categories}
}
func(a *API) currentSettings()(store.BoxSettings,error){
 if a.Store!=nil{v,ok,err:=a.Store.LoadBoxSettings();if err!=nil{return store.BoxSettings{},err};if ok{return v,nil}}
 status:=a.Player.Status()
 return store.BoxSettings{Language:"de",AdminLanguage:"de",TTS:store.TTSSettings{Enabled:a.TTS.Enabled,Language:a.TTS.Language,Provider:a.TTS.Provider},Power:store.PowerSettings{IdleShutdownMinutes:a.Power.IdleShutdownMinutes},Audio:store.AudioSettings{StartupVolume:min(30,status.MaxVolume),MaxVolume:status.MaxVolume},Display:store.DisplaySettings{Brightness:100,UISize:"normal"},Bluetooth:store.BluetoothSettings{Enabled:false},Theme:"modern-dark"},nil
}
func(a *API)wifiAdapters()([]connectivity.WiFiAdapter,string,string,error){
 if a.Connectivity==nil{return nil,"","",fmt.Errorf("connectivity manager unavailable")}
 adapters,err:=a.Connectivity.ListWiFiAdapters();if err!=nil{return nil,"","",err}
 settings,err:=a.currentSettings();if err!=nil{return nil,"","",err}
 disabled:=map[string]bool{};for _,name:=range settings.WiFi.DisabledInterfaces{disabled[name]=true}
 configured:=strings.TrimSpace(settings.WiFi.PrimaryInterface);selected:=""
 for index:=range adapters{adapters[index].Enabled=!disabled[adapters[index].Interface];adapters[index].Usable=adapters[index].Usable&&adapters[index].Enabled;if configured!=""&&adapters[index].Interface==configured{adapters[index].Preferred=true;if adapters[index].Usable{selected=configured}}}
 if selected==""{for _,adapter:=range adapters{if adapter.Enabled&&adapter.Usable&&adapter.State=="up"{selected=adapter.Interface;break}}}
 if selected==""{for _,adapter:=range adapters{if adapter.Enabled&&adapter.Usable{selected=adapter.Interface;break}}}
 for index:=range adapters{adapters[index].Selected=adapters[index].Interface==selected;if configured==""&&adapters[index].Selected{adapters[index].Preferred=true}}
 primary:=configured;if primary==""{primary=selected}
 return adapters,primary,selected,nil
}
func(a *API)wifiAdapter(name string)(connectivity.WiFiAdapter,error){adapters,_,_,err:=a.wifiAdapters();if err!=nil{return connectivity.WiFiAdapter{},err};for _,adapter:=range adapters{if adapter.Interface==name{if !adapter.Enabled{return adapter,fmt.Errorf("Wi-Fi adapter %s is disabled for MuPiBox",name)};if !adapter.Usable{return adapter,fmt.Errorf("Wi-Fi adapter %s is not ready",name)};return adapter,nil}};return connectivity.WiFiAdapter{},fmt.Errorf("unknown Wi-Fi adapter %s",name)}
func(a *API) Handler()http.Handler{
 mux:=http.NewServeMux()
 mux.HandleFunc("GET /api/status",func(w http.ResponseWriter,r *http.Request){jsonResponse(w,200,a.Player.Status())})
 mux.HandleFunc("GET /api/system",func(w http.ResponseWriter,r *http.Request){jsonResponse(w,200,a.currentSystemStatus())})
 mux.HandleFunc("GET /api/connectivity/wifi/adapters",func(w http.ResponseWriter,r *http.Request){adapters,primary,selected,err:=a.wifiAdapters();if err!=nil{problem(w,503,err);return};jsonResponse(w,200,map[string]any{"adapters":adapters,"primary_interface":primary,"selected_interface":selected})})
 mux.HandleFunc("PUT /api/connectivity/wifi/preferences",func(w http.ResponseWriter,r *http.Request){if a.Store==nil||a.Connectivity==nil{problem(w,503,fmt.Errorf("persistent connectivity settings unavailable"));return};var preferences store.WiFiSettings;if err:=decode(w,r,&preferences);err!=nil{problem(w,400,err);return};detected,err:=a.Connectivity.ListWiFiAdapters();if err!=nil{problem(w,503,err);return};disabled:=map[string]bool{};for _,name:=range preferences.DisabledInterfaces{disabled[name]=true};ready:=0;primaryKnown:=preferences.PrimaryInterface=="";for _,adapter:=range detected{if adapter.Interface==preferences.PrimaryInterface{primaryKnown=true};if !disabled[adapter.Interface]&&adapter.Usable{ready++}};if !primaryKnown{problem(w,400,fmt.Errorf("preferred Wi-Fi adapter was not detected"));return};if ready==0{problem(w,400,fmt.Errorf("at least one enabled and ready Wi-Fi adapter is required"));return};settings,ok,err:=a.Store.LoadBoxSettings();if err!=nil||!ok{if err==nil{err=fmt.Errorf("settings not initialized")};problem(w,500,err);return};settings.WiFi=preferences;if err=a.Store.SaveBoxSettings(settings);err!=nil{problem(w,400,err);return};adapters,primary,selected,err:=a.wifiAdapters();if err!=nil{problem(w,500,err);return};jsonResponse(w,200,map[string]any{"adapters":adapters,"primary_interface":primary,"selected_interface":selected})})
 mux.HandleFunc("GET /api/connectivity/wifi",func(w http.ResponseWriter,r *http.Request){if a.Connectivity==nil{problem(w,503,fmt.Errorf("connectivity manager unavailable"));return};iface:=strings.TrimSpace(r.URL.Query().Get("interface"));if iface==""||iface=="all"||iface=="auto"{_,_,selected,err:=a.wifiAdapters();if err!=nil{problem(w,503,err);return};iface=selected;if iface==""{problem(w,409,fmt.Errorf("no enabled and ready Wi-Fi adapter found"));return}}else if _,err:=a.wifiAdapter(iface);err!=nil{problem(w,409,err);return};networks,err:=a.Connectivity.ScanWiFiOn(r.Context(),iface);if err!=nil{problem(w,503,err);return};jsonResponse(w,200,map[string]any{"interface":iface,"networks":networks})})
 mux.HandleFunc("POST /api/connectivity/wifi/connect",func(w http.ResponseWriter,r *http.Request){if a.Connectivity==nil{problem(w,503,fmt.Errorf("connectivity manager unavailable"));return};var request connectivity.WiFiConnectRequest;if err:=decode(w,r,&request);err!=nil{problem(w,400,err);return};request.Interface=strings.TrimSpace(request.Interface);if request.Interface==""||request.Interface=="auto"||request.Interface=="all"{_,_,selected,err:=a.wifiAdapters();if err!=nil{problem(w,503,err);return};request.Interface=selected};if _,err:=a.wifiAdapter(request.Interface);err!=nil{problem(w,409,err);return};if err:=a.Connectivity.ConnectWiFi(r.Context(),request);err!=nil{problem(w,502,err);return};jsonResponse(w,200,map[string]bool{"connected":true})})
 mux.HandleFunc("GET /api/connectivity/bluetooth",func(w http.ResponseWriter,r *http.Request){if a.Connectivity==nil{problem(w,503,fmt.Errorf("connectivity manager unavailable"));return};settings,err:=a.currentSettings();if err!=nil{problem(w,500,err);return};if !settings.Bluetooth.Enabled{problem(w,409,fmt.Errorf("Bluetooth is disabled"));return};devices,err:=a.Connectivity.ScanBluetooth(r.Context());if err!=nil{problem(w,503,err);return};jsonResponse(w,200,map[string]any{"enabled":true,"devices":devices})})
 mux.HandleFunc("POST /api/connectivity/bluetooth/command",func(w http.ResponseWriter,r *http.Request){if a.Connectivity==nil{problem(w,503,fmt.Errorf("connectivity manager unavailable"));return};settings,err:=a.currentSettings();if err!=nil{problem(w,500,err);return};if !settings.Bluetooth.Enabled{problem(w,409,fmt.Errorf("Bluetooth is disabled"));return};var request struct{Action string `json:"action"`;Address string `json:"address"`};if err=decode(w,r,&request);err!=nil{problem(w,400,err);return};if err=a.Connectivity.BluetoothCommand(r.Context(),request.Action,request.Address);err!=nil{problem(w,502,err);return};jsonResponse(w,200,map[string]bool{"ok":true})})
 mux.HandleFunc("GET /api/ui-state",func(w http.ResponseWriter,r *http.Request){jsonResponse(w,200,map[string]uint64{"restart_generation":a.uiRestartGeneration.Load()})})
 mux.HandleFunc("GET /api/library",func(w http.ResponseWriter,r *http.Request){lib:=a.librarySnapshot();if lib==nil{problem(w,503,fmt.Errorf("library unavailable"));return};jsonResponse(w,200,lib.Folders)})
 mux.HandleFunc("GET /api/home",func(w http.ResponseWriter,r *http.Request){jsonResponse(w,200,a.Home())})
 mux.HandleFunc("GET /api/info",func(w http.ResponseWriter,r *http.Request){
  settings,err:=a.currentSettings();if err!=nil{problem(w,500,err);return}
  jsonResponse(w,200,map[string]any{"version":a.Version,"simulation":a.Inputs.Simulate,"backend":a.Player.Status().Backend,"tts":settings.TTS,"power":settings.Power,"display":settings.Display,"theme":settings.Theme,"settings_persistent":a.Store!=nil})
 })
 mux.HandleFunc("GET /api/health",func(w http.ResponseWriter,r *http.Request){jsonResponse(w,200,map[string]string{"status":"ok","version":a.Version})})
 mux.HandleFunc("GET /api/admin/auth",func(w http.ResponseWriter,r *http.Request){_,protected,err:=a.adminPasswordHash();if err!=nil{problem(w,500,err);return};jsonResponse(w,200,map[string]bool{"protected":protected,"authenticated":!protected||a.adminAuthenticated(r)})})
 mux.HandleFunc("POST /api/admin/login",func(w http.ResponseWriter,r *http.Request){if !a.loginAllowed(r){problem(w,http.StatusTooManyRequests,fmt.Errorf("too many login attempts; try again later"));return};var request struct{Password string `json:"password"`};if err:=decode(w,r,&request);err!=nil{problem(w,400,err);return};encoded,protected,err:=a.adminPasswordHash();if err!=nil{problem(w,500,err);return};if !protected{problem(w,409,fmt.Errorf("admin password is not configured"));return};if !verifyAdminPassword(encoded,request.Password){a.recordLogin(r,false);problem(w,401,fmt.Errorf("invalid password"));return};a.recordLogin(r,true);if err=a.newAdminSession(w,r);err!=nil{problem(w,500,err);return};jsonResponse(w,200,map[string]bool{"authenticated":true})})
 mux.HandleFunc("POST /api/admin/logout",func(w http.ResponseWriter,r *http.Request){a.clearAdminSessions(w,r);jsonResponse(w,200,map[string]bool{"authenticated":false})})
 mux.HandleFunc("PUT /api/admin/password",func(w http.ResponseWriter,r *http.Request){if a.Store==nil{problem(w,503,fmt.Errorf("persistent store unavailable"));return};var request struct{CurrentPassword string `json:"current_password"`;NewPassword string `json:"new_password"`};if err:=decode(w,r,&request);err!=nil{problem(w,400,err);return};current,protected,err:=a.adminPasswordHash();if err!=nil{problem(w,500,err);return};if protected&&!verifyAdminPassword(current,request.CurrentPassword){problem(w,403,fmt.Errorf("current password is invalid"));return};encoded,err:=hashAdminPassword(request.NewPassword);if err!=nil{problem(w,400,err);return};if err=a.Store.SaveAdminPasswordHash(encoded);err!=nil{problem(w,500,err);return};a.clearAdminSessions(w,r);if err=a.newAdminSession(w,r);err!=nil{problem(w,500,err);return};jsonResponse(w,200,map[string]bool{"protected":true,"authenticated":true})})
 mux.HandleFunc("GET /api/admin/settings",func(w http.ResponseWriter,r *http.Request){if a.Store==nil{problem(w,503,fmt.Errorf("persistent store unavailable"));return};v,ok,err:=a.Store.LoadBoxSettings();if err!=nil{problem(w,500,err);return};if !ok{problem(w,404,fmt.Errorf("settings not initialized"));return};jsonResponse(w,200,v)})
 mux.HandleFunc("PUT /api/admin/settings",func(w http.ResponseWriter,r *http.Request){if a.Store==nil{problem(w,503,fmt.Errorf("persistent store unavailable"));return};var v store.BoxSettings;if err:=decode(w,r,&v);err!=nil{problem(w,400,err);return};current,_,err:=a.Store.LoadBoxSettings();if err!=nil{problem(w,500,err);return};if current.Bluetooth.Enabled!=v.Bluetooth.Enabled{if a.Connectivity==nil{problem(w,503,fmt.Errorf("connectivity manager unavailable"));return};if err=a.Connectivity.SetBluetoothPower(r.Context(),v.Bluetooth.Enabled);err!=nil{problem(w,502,err);return}};if err=a.Store.SaveBoxSettings(v);err!=nil{problem(w,400,err);return};a.TTS=TTSConfig{Enabled:v.TTS.Enabled,Language:v.TTS.Language,Provider:v.TTS.Provider};a.Power=PowerConfig{IdleShutdownMinutes:v.Power.IdleShutdownMinutes};jsonResponse(w,200,map[string]any{"settings":v,"restart_required":[]string{"audio.max_volume","audio.startup_volume"}})})
 mux.HandleFunc("GET /api/admin/navigation",func(w http.ResponseWriter,r *http.Request){if a.Store==nil{problem(w,503,fmt.Errorf("persistent store unavailable"));return};v,err:=a.Store.LoadNavigation();if err!=nil{problem(w,500,err);return};jsonResponse(w,200,v)})
 mux.HandleFunc("GET /api/admin/local-directories",func(w http.ResponseWriter,r *http.Request){directories,err:=a.localDirectories();if err!=nil{problem(w,500,err);return};lib:=a.librarySnapshot();jsonResponse(w,200,map[string]any{"root":lib.Root,"directories":directories})})
 mux.HandleFunc("POST /api/admin/library/rescan",func(w http.ResponseWriter,r *http.Request){if err:=a.rescanLibrary();err!=nil{problem(w,500,err);return};directories,err:=a.localDirectories();if err!=nil{problem(w,500,err);return};jsonResponse(w,200,map[string]any{"directories":directories,"folders":len(a.librarySnapshot().Folders)})})
 mux.HandleFunc("POST /api/admin/ui/restart",func(w http.ResponseWriter,r *http.Request){generation:=a.uiRestartGeneration.Add(1);jsonResponse(w,202,map[string]uint64{"restart_generation":generation})})
 mux.HandleFunc("PUT /api/admin/navigation",func(w http.ResponseWriter,r *http.Request){if a.Store==nil{problem(w,503,fmt.Errorf("persistent store unavailable"));return};var v store.Navigation;if err:=decode(w,r,&v);err!=nil{problem(w,400,err);return};if err:=a.rescanLibrary();err!=nil{problem(w,500,err);return};if err:=a.Store.SaveNavigation(v);err!=nil{problem(w,400,err);return};jsonResponse(w,200,v)})
 mux.HandleFunc("GET /api/cover/{id}",func(w http.ResponseWriter,r *http.Request){
  lib:=a.librarySnapshot();if lib==nil{http.NotFound(w,r);return};f,ok:=lib.Folder(r.PathValue("id"));if !ok||f.CoverPath==""{http.NotFound(w,r);return}
  if err:=lib.Validate(f.CoverPath);err!=nil{http.NotFound(w,r);return};http.ServeFile(w,r,f.CoverPath)
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
  if !a.authorizeRequest(w,r){return}
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
