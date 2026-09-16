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
 "mupibox/webui"
)

type InputConfig struct {
 Simulate bool `json:"simulate"`
 Buttons map[string]core.Command `json:"buttons"`
 RFID map[string]string `json:"rfid"` // UID -> folder ID (M1 only)
}
type Input struct { Module string `json:"module"`; ID string `json:"id"` }
type API struct { Player *core.Controller; Library *library.Library; Inputs InputConfig; Version string }
func jsonResponse(w http.ResponseWriter,status int,v any){w.Header().Set("Content-Type","application/json");w.Header().Set("Cache-Control","no-store");w.WriteHeader(status);_ = json.NewEncoder(w).Encode(v)}
func problem(w http.ResponseWriter,status int,err error){jsonResponse(w,status,map[string]string{"error":err.Error()})}
func decode(w http.ResponseWriter,r *http.Request,v any)error{
 if !strings.HasPrefix(r.Header.Get("Content-Type"),"application/json"){return fmt.Errorf("Content-Type must be application/json")}
 r.Body=http.MaxBytesReader(w,r.Body,8192);d:=json.NewDecoder(r.Body);d.DisallowUnknownFields()
 if err:=d.Decode(v);err!=nil{return err};var extra any;if err:=d.Decode(&extra);err!=io.EOF{return fmt.Errorf("expected one JSON object")};return nil
}
func(a *API) Handler()http.Handler{
 mux:=http.NewServeMux()
 mux.HandleFunc("GET /api/status",func(w http.ResponseWriter,r *http.Request){jsonResponse(w,200,a.Player.Status())})
 mux.HandleFunc("GET /api/library",func(w http.ResponseWriter,r *http.Request){jsonResponse(w,200,a.Library.Folders)})
 mux.HandleFunc("GET /api/info",func(w http.ResponseWriter,r *http.Request){jsonResponse(w,200,map[string]any{"version":a.Version,"simulation":a.Inputs.Simulate,"backend":a.Player.Status().Backend})})
 mux.HandleFunc("GET /api/health",func(w http.ResponseWriter,r *http.Request){jsonResponse(w,200,map[string]string{"status":"ok","version":a.Version})})
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
  w.Header().Set("Content-Security-Policy","default-src 'self'; img-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
  // Browser callers may mutate only from the same origin; JSON prevents form posts.
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
