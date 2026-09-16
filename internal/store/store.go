// Package store owns MuPiBox's SQLite persistence and schema migrations.
package store

import (
 "database/sql"
 "encoding/json"
 "errors"
 "fmt"
 "os"
 "path/filepath"
 "regexp"
 "strings"
 "time"

 _ "github.com/mattn/go-sqlite3"
)

type Store struct{ db *sql.DB }

type TTSSettings struct {
 Enabled bool `json:"enabled"`
 Language string `json:"language"`
 Provider string `json:"provider"`
}

type PowerSettings struct {
 IdleShutdownMinutes int `json:"idle_shutdown_minutes"`
}

type AudioSettings struct {
 StartupVolume int `json:"startup_volume"`
 MaxVolume int `json:"max_volume"`
 StartSoundEnabled bool `json:"start_sound_enabled"`
 ShutdownSoundEnabled bool `json:"shutdown_sound_enabled"`
}

type DisplaySettings struct {
 IdleOffMinutes int `json:"idle_off_minutes"`
 Brightness int `json:"brightness"`
 UISize string `json:"ui_size"`
}

type BoxSettings struct {
 Language string `json:"language"`
 AdminLanguage string `json:"admin_language"`
 TTS TTSSettings `json:"tts"`
 Power PowerSettings `json:"power"`
 Audio AudioSettings `json:"audio"`
 Display DisplaySettings `json:"display"`
 Theme string `json:"theme"`
}

type Navigation struct{ Categories []Category `json:"categories"` }
type Category struct {
 ID string `json:"id"`
 Labels map[string]string `json:"labels"`
 Rows []Row `json:"rows,omitempty"`
}
type Row struct {
 ID string `json:"id"`
 Labels map[string]string `json:"labels"`
 Provider string `json:"provider"`
 SourceType string `json:"source_type,omitempty"`
 SourceRef string `json:"source_ref,omitempty"`
}

type Progress struct {
 Provider string `json:"provider"`
 AccountID string `json:"account_id,omitempty"`
 MediaID string `json:"media_id"`
 PositionMS int64 `json:"position_ms"`
 DurationMS int64 `json:"duration_ms"`
 ContextID string `json:"context_id,omitempty"`
 ItemIndex int `json:"item_index,omitempty"`
 Completed bool `json:"completed"`
 UpdatedAt time.Time `json:"updated_at"`
}

func Open(path string)(*Store,error){
 if strings.TrimSpace(path)==""{return nil,errors.New("database path is required")}
 if path!=":memory:"&&!strings.HasPrefix(path,"file:"){
  if err:=os.MkdirAll(filepath.Dir(path),0750);err!=nil{return nil,fmt.Errorf("create database directory: %w",err)}
 }
 db,err:=sql.Open("sqlite3",path);if err!=nil{return nil,err}
 db.SetMaxOpenConns(1)
 s:=&Store{db:db}
 if err=s.migrate();err!=nil{_ = db.Close();return nil,err}
 return s,nil
}
func(s *Store)Close()error{return s.db.Close()}

func(s *Store)migrate()error{
 if _,err:=s.db.Exec(`PRAGMA foreign_keys=ON`);err!=nil{return err}
 if _,err:=s.db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations(version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`);err!=nil{return err}
 var version int
 if err:=s.db.QueryRow(`SELECT COALESCE(MAX(version),0) FROM schema_migrations`).Scan(&version);err!=nil{return err}
 if version<1{
  tx,err:=s.db.Begin();if err!=nil{return err};defer tx.Rollback()
  statements:=[]string{
   `CREATE TABLE settings(key TEXT PRIMARY KEY, value TEXT NOT NULL, updated_at TEXT NOT NULL)`,
   `CREATE TABLE navigation_nodes(
     id TEXT PRIMARY KEY,
     parent_id TEXT REFERENCES navigation_nodes(id) ON DELETE CASCADE,
     node_type TEXT NOT NULL CHECK(node_type IN ('category','row')),
     sort_order INTEGER NOT NULL,
     enabled INTEGER NOT NULL DEFAULT 1 CHECK(enabled IN (0,1)),
     provider TEXT NOT NULL DEFAULT ''
   )`,
   `CREATE INDEX navigation_parent_sort ON navigation_nodes(parent_id,sort_order)`,
   `CREATE TABLE navigation_labels(
     node_id TEXT NOT NULL REFERENCES navigation_nodes(id) ON DELETE CASCADE,
     language TEXT NOT NULL,
     label TEXT NOT NULL,
     PRIMARY KEY(node_id,language)
   )`,
   `CREATE TABLE playback_progress(
     provider TEXT NOT NULL,
     account_id TEXT NOT NULL DEFAULT '',
     media_id TEXT NOT NULL,
     position_ms INTEGER NOT NULL CHECK(position_ms>=0),
     duration_ms INTEGER NOT NULL CHECK(duration_ms>=0),
     context_id TEXT NOT NULL DEFAULT '',
     item_index INTEGER NOT NULL DEFAULT 0,
     completed INTEGER NOT NULL DEFAULT 0 CHECK(completed IN (0,1)),
     updated_at TEXT NOT NULL,
     PRIMARY KEY(provider,account_id,media_id)
   )`,
  }
  for _,q:=range statements{if _,err=tx.Exec(q);err!=nil{return fmt.Errorf("migration 1: %w",err)}}
  if _,err=tx.Exec(`INSERT INTO schema_migrations(version,applied_at) VALUES(1,?)`,time.Now().UTC().Format(time.RFC3339Nano));err!=nil{return err}
  if err=tx.Commit();err!=nil{return err};version=1
 }
 if version<2{
  tx,err:=s.db.Begin();if err!=nil{return err};defer tx.Rollback()
  if _,err=tx.Exec(`ALTER TABLE navigation_nodes ADD COLUMN source_type TEXT NOT NULL DEFAULT ''`);err!=nil{return fmt.Errorf("migration 2 source_type: %w",err)}
  if _,err=tx.Exec(`ALTER TABLE navigation_nodes ADD COLUMN source_ref TEXT NOT NULL DEFAULT ''`);err!=nil{return fmt.Errorf("migration 2 source_ref: %w",err)}
  if _,err=tx.Exec(`INSERT INTO schema_migrations(version,applied_at) VALUES(2,?)`,time.Now().UTC().Format(time.RFC3339Nano));err!=nil{return err}
  if err=tx.Commit();err!=nil{return err}
 }
 return nil
}

func ValidateBoxSettings(v BoxSettings)error{
 if strings.TrimSpace(v.Language)==""{return errors.New("language is required")}
 if strings.TrimSpace(v.AdminLanguage)==""{return errors.New("admin_language is required")}
 if v.Audio.MaxVolume<1||v.Audio.MaxVolume>100{return errors.New("audio.max_volume must be 1..100")}
 if v.Audio.StartupVolume<0||v.Audio.StartupVolume>v.Audio.MaxVolume{return errors.New("audio.startup_volume must be between 0 and max_volume")}
 if v.Power.IdleShutdownMinutes<0{return errors.New("power.idle_shutdown_minutes must be >= 0")}
 if v.Display.IdleOffMinutes<0{return errors.New("display.idle_off_minutes must be >= 0")}
 if v.Display.Brightness<1||v.Display.Brightness>100{return errors.New("display.brightness must be 1..100")}
 if v.Display.UISize!="normal"&&v.Display.UISize!="large"{return errors.New("display.ui_size must be normal or large")}
 if v.TTS.Enabled&&(strings.TrimSpace(v.TTS.Language)==""||strings.TrimSpace(v.TTS.Provider)==""){return errors.New("enabled TTS requires language and provider")}
 if strings.TrimSpace(v.Theme)==""{return errors.New("theme is required")}
 return nil
}
func(s *Store)LoadBoxSettings()(BoxSettings,bool,error){
 var raw string
 err:=s.db.QueryRow(`SELECT value FROM settings WHERE key='box.settings'`).Scan(&raw)
 if errors.Is(err,sql.ErrNoRows){return BoxSettings{},false,nil};if err!=nil{return BoxSettings{},false,err}
 var v BoxSettings;if err=json.Unmarshal([]byte(raw),&v);err!=nil{return BoxSettings{},false,fmt.Errorf("decode box settings: %w",err)}
 if strings.TrimSpace(v.AdminLanguage)==""{v.AdminLanguage=v.Language;if strings.TrimSpace(v.AdminLanguage)==""{v.AdminLanguage="de"}}
 if strings.TrimSpace(v.Display.UISize)==""{v.Display.UISize="normal"}
 return v,true,nil
}
func(s *Store)SaveBoxSettings(v BoxSettings)error{
 if strings.TrimSpace(v.Display.UISize)==""{v.Display.UISize="normal"}
 if err:=ValidateBoxSettings(v);err!=nil{return err}
 raw,err:=json.Marshal(v);if err!=nil{return err}
 _,err=s.db.Exec(`INSERT INTO settings(key,value,updated_at) VALUES('box.settings',?,?)
 ON CONFLICT(key) DO UPDATE SET value=excluded.value,updated_at=excluded.updated_at`,string(raw),time.Now().UTC().Format(time.RFC3339Nano))
 return err
}
func(s *Store)EnsureBoxSettings(defaults BoxSettings)(BoxSettings,error){
 v,ok,err:=s.LoadBoxSettings();if err!=nil{return BoxSettings{},err};if ok{return v,nil}
 if err= s.SaveBoxSettings(defaults);err!=nil{return BoxSettings{},err};return defaults,nil
}

var validID=regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
func validateLabels(labels map[string]string)error{
 if len(labels)==0{return errors.New("at least one label is required")}
 for lang,label:=range labels{
  if !validID.MatchString(lang){return fmt.Errorf("invalid language %q",lang)}
  if strings.TrimSpace(label)==""||len(label)>160{return fmt.Errorf("invalid label for %s",lang)}
 }
 return nil
}
func ValidateNavigation(v Navigation)error{
 seen:=map[string]bool{}
 for _,c:=range v.Categories{
  if !validID.MatchString(c.ID){return fmt.Errorf("invalid category id %q",c.ID)}
  if seen[c.ID]{return fmt.Errorf("duplicate node id %q",c.ID)};seen[c.ID]=true
  if err:=validateLabels(c.Labels);err!=nil{return fmt.Errorf("category %s: %w",c.ID,err)}
  for _,r:=range c.Rows{
   if !validID.MatchString(r.ID){return fmt.Errorf("invalid media id %q",r.ID)}
   if seen[r.ID]{return fmt.Errorf("duplicate node id %q",r.ID)};seen[r.ID]=true
   if err:=validateLabels(r.Labels);err!=nil{return fmt.Errorf("media %s: %w",r.ID,err)}
   if strings.TrimSpace(r.Provider)==""{return fmt.Errorf("media %s: provider is required",r.ID)}
   if r.SourceType!=""&&!validID.MatchString(r.SourceType){return fmt.Errorf("media %s: invalid source_type",r.ID)}
   if len(r.SourceRef)>2048{return fmt.Errorf("media %s: source_ref is too long",r.ID)}
   if r.Provider=="local-library"&&r.SourceType=="path"{
    ref:=filepath.Clean(filepath.FromSlash(strings.TrimSpace(r.SourceRef)))
    if strings.TrimSpace(r.SourceRef)==""||filepath.IsAbs(ref)||ref==".."||strings.HasPrefix(ref,".."+string(filepath.Separator)){return fmt.Errorf("media %s: local path must be below the media root",r.ID)}
   }
  }
 }
 return nil
}
func(s *Store)HasNavigation()(bool,error){var n int;err:=s.db.QueryRow(`SELECT COUNT(*) FROM navigation_nodes`).Scan(&n);return n>0,err}
func(s *Store)SaveNavigation(v Navigation)error{
 if err:=ValidateNavigation(v);err!=nil{return err}
 tx,err:=s.db.Begin();if err!=nil{return err};defer tx.Rollback()
 if _,err=tx.Exec(`DELETE FROM navigation_nodes`);err!=nil{return err}
 insertNode:=func(id string,parent any,kind string,order int,provider,sourceType,sourceRef string)error{
  _,err:=tx.Exec(`INSERT INTO navigation_nodes(id,parent_id,node_type,sort_order,enabled,provider,source_type,source_ref) VALUES(?,?,?,?,1,?,?,?)`,id,parent,kind,order,provider,sourceType,sourceRef);return err
 }
 insertLabels:=func(id string,labels map[string]string)error{for lang,label:=range labels{if _,err:=tx.Exec(`INSERT INTO navigation_labels(node_id,language,label) VALUES(?,?,?)`,id,lang,label);err!=nil{return err}};return nil}
 for ci,c:=range v.Categories{
  if err=insertNode(c.ID,nil,"category",ci,"","","");err!=nil{return err};if err=insertLabels(c.ID,c.Labels);err!=nil{return err}
  for ri,r:=range c.Rows{if err=insertNode(r.ID,c.ID,"row",ri,r.Provider,r.SourceType,r.SourceRef);err!=nil{return err};if err=insertLabels(r.ID,r.Labels);err!=nil{return err}}
 }
 return tx.Commit()
}
func(s *Store)EnsureNavigation(defaults Navigation)error{ok,err:=s.HasNavigation();if err!=nil||ok{return err};return s.SaveNavigation(defaults)}
func(s *Store)LoadNavigation()(Navigation,error){
 rows,err:=s.db.Query(`SELECT id,parent_id,node_type,provider,source_type,source_ref FROM navigation_nodes WHERE enabled=1 ORDER BY CASE WHEN parent_id IS NULL THEN 0 ELSE 1 END,sort_order,id`);if err!=nil{return Navigation{},err};defer rows.Close()
 type node struct{id string;parent sql.NullString;kind,provider,sourceType,sourceRef string};nodes:=[]node{}
 for rows.Next(){var n node;if err=rows.Scan(&n.id,&n.parent,&n.kind,&n.provider,&n.sourceType,&n.sourceRef);err!=nil{return Navigation{},err};nodes=append(nodes,n)}
 if err=rows.Err();err!=nil{return Navigation{},err}
 labels:=map[string]map[string]string{}
 lr,err:=s.db.Query(`SELECT node_id,language,label FROM navigation_labels`);if err!=nil{return Navigation{},err};defer lr.Close()
 for lr.Next(){var id,lang,label string;if err=lr.Scan(&id,&lang,&label);err!=nil{return Navigation{},err};if labels[id]==nil{labels[id]=map[string]string{}};labels[id][lang]=label}
 out:=Navigation{Categories:[]Category{}};indexes:=map[string]int{}
 for _,n:=range nodes{if n.kind=="category"{indexes[n.id]=len(out.Categories);out.Categories=append(out.Categories,Category{ID:n.id,Labels:labels[n.id],Rows:[]Row{}})}}
 for _,n:=range nodes{if n.kind!="row"||!n.parent.Valid{continue};i,ok:=indexes[n.parent.String];if ok{out.Categories[i].Rows=append(out.Categories[i].Rows,Row{ID:n.id,Labels:labels[n.id],Provider:n.provider,SourceType:n.sourceType,SourceRef:n.sourceRef})}}
 return out,lr.Err()
}

func(s *Store)SaveProgress(v Progress)error{
 if strings.TrimSpace(v.Provider)==""||strings.TrimSpace(v.MediaID)==""{return errors.New("provider and media_id are required")}
 if v.Provider=="radio"{return nil}
 if v.PositionMS<0||v.DurationMS<0{return errors.New("progress values must be >= 0")}
 now:=time.Now().UTC();if !v.UpdatedAt.IsZero(){now=v.UpdatedAt.UTC()}
 _,err:=s.db.Exec(`INSERT INTO playback_progress(provider,account_id,media_id,position_ms,duration_ms,context_id,item_index,completed,updated_at)
 VALUES(?,?,?,?,?,?,?,?,?)
 ON CONFLICT(provider,account_id,media_id) DO UPDATE SET position_ms=excluded.position_ms,duration_ms=excluded.duration_ms,
 context_id=excluded.context_id,item_index=excluded.item_index,completed=excluded.completed,updated_at=excluded.updated_at`,
 v.Provider,v.AccountID,v.MediaID,v.PositionMS,v.DurationMS,v.ContextID,v.ItemIndex,v.Completed,now.Format(time.RFC3339Nano))
 return err
}
func(s *Store)LoadProgress(provider,accountID,mediaID string)(Progress,bool,error){
 var v Progress;var completed int;var updated string
 err:=s.db.QueryRow(`SELECT position_ms,duration_ms,context_id,item_index,completed,updated_at FROM playback_progress WHERE provider=? AND account_id=? AND media_id=?`,provider,accountID,mediaID).Scan(&v.PositionMS,&v.DurationMS,&v.ContextID,&v.ItemIndex,&completed,&updated)
 if errors.Is(err,sql.ErrNoRows){return Progress{},false,nil};if err!=nil{return Progress{},false,err}
 v.Provider=provider;v.AccountID=accountID;v.MediaID=mediaID;v.Completed=completed==1;v.UpdatedAt,_=time.Parse(time.RFC3339Nano,updated)
 return v,true,nil
}

func(s *Store)PutProgress(provider,accountID,mediaID string,positionMS,durationMS int64,contextID string,itemIndex int,completed bool)error{
 return s.SaveProgress(Progress{Provider:provider,AccountID:accountID,MediaID:mediaID,PositionMS:positionMS,DurationMS:durationMS,ContextID:contextID,ItemIndex:itemIndex,Completed:completed})
}
func(s *Store)GetProgress(provider,accountID,mediaID string)(positionMS,durationMS int64,completed,found bool,err error){
 v,found,err:=s.LoadProgress(provider,accountID,mediaID);if err!=nil||!found{return 0,0,false,found,err}
 return v.PositionMS,v.DurationMS,v.Completed,true,nil
}
