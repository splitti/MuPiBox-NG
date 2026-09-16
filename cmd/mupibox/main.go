package main

import (
 "context"
 "encoding/json"
 "errors"
 "flag"
 "fmt"
 "io"
 "log"
 "net/http"
 "os"
 "os/signal"
 "syscall"
 "time"

 "mupibox/internal/audio"
 "mupibox/internal/core"
 "mupibox/internal/library"
 "mupibox/internal/server"
)
var version="0.1.0-dev"
type config struct {
 Listen string `json:"listen"`
 MusicDir string `json:"music_dir"`
 Backend string `json:"backend"`
 MaxVolume int `json:"max_volume"`
 Inputs server.InputConfig `json:"inputs"`
}
func run()error{
 path:=flag.String("config","","JSON configuration file")
 showVersion:=flag.Bool("version",false,"print version")
 flag.Parse();if *showVersion{fmt.Println(version);return nil}
 cfg:=config{Listen:"127.0.0.1:8080",MusicDir:"./music",Backend:"mpv",MaxVolume:60}
 if *path!=""{
  f,err:=os.Open(*path);if err!=nil{return err};defer f.Close()
  d:=json.NewDecoder(f);d.DisallowUnknownFields();if err=d.Decode(&cfg);err!=nil{return err};var extra any;if d.Decode(&extra)!=io.EOF{return errors.New("expected one config object")}
 }
 lib,err:=library.Scan(cfg.MusicDir);if err!=nil{return fmt.Errorf("scan music: %w",err)}
 var backend audio.Backend
 switch cfg.Backend{case "mpv":backend=&audio.MPV{};case "simulated":backend=&audio.Simulated{};default:return fmt.Errorf("unknown backend %q",cfg.Backend)}
 p,err:=core.New(lib,backend,cfg.MaxVolume);if err!=nil{return err};defer p.Close()
 ctx,cancel:=signal.NotifyContext(context.Background(),os.Interrupt,syscall.SIGTERM);defer cancel()
 workerDone:=make(chan struct{});go func(){defer close(workerDone);p.Run(ctx)}()
 defer func(){cancel();<-workerDone}()
 api:=&server.API{Player:p,Library:lib,Inputs:cfg.Inputs,Version:version}
 httpServer:=&http.Server{Addr:cfg.Listen,Handler:api.Handler(),ReadHeaderTimeout:5*time.Second,ReadTimeout:10*time.Second,WriteTimeout:30*time.Second,IdleTimeout:60*time.Second}
 shutdownDone:=make(chan struct{});go func(){defer close(shutdownDone);<-ctx.Done();c,stop:=context.WithTimeout(context.Background(),15*time.Second);defer stop();if err:=httpServer.Shutdown(c);err!=nil{_ = httpServer.Close()}}()
 log.Printf("MuPiBox %s: http://%s, backend=%s, folders=%d",version,cfg.Listen,backend.Name(),len(lib.Folders))
 err=httpServer.ListenAndServe();cancel();<-shutdownDone
 if err!=nil&&!errors.Is(err,http.ErrServerClosed){return err};return nil
}
func main(){if err:=run();err!=nil{log.Fatal(err)}}
