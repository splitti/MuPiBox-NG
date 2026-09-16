package server

import (
 "bufio"
 "io"
 "math"
 "os"
 "strconv"
 "strings"
)

type WiFiStatus struct {
 Connected bool `json:"connected"`
 Interface string `json:"interface,omitempty"`
 SignalDBM int `json:"signal_dbm,omitempty"`
 QualityPercent int `json:"quality_percent,omitempty"`
}

type SystemStatus struct {
 WiFi WiFiStatus `json:"wifi"`
}

func (a *API) currentSystemStatus() SystemStatus {
 if a.System!=nil{return a.System()}
 return readSystemStatus()
}

func readSystemStatus() SystemStatus {
 file,err:=os.Open("/proc/net/wireless")
 if err!=nil{return SystemStatus{}}
 defer file.Close()
 return SystemStatus{WiFi:parseWiFiStatus(file)}
}

func parseWiFiStatus(reader io.Reader) WiFiStatus {
 scanner:=bufio.NewScanner(reader)
 for scanner.Scan(){
  line:=strings.TrimSpace(scanner.Text())
  colon:=strings.IndexByte(line,':')
  if colon<1{continue}
  name:=strings.TrimSpace(line[:colon])
  fields:=strings.Fields(line[colon+1:])
  if len(fields)<3{continue}
  link,linkErr:=strconv.ParseFloat(strings.TrimSuffix(fields[1],"."),64)
  level,levelErr:=strconv.ParseFloat(strings.TrimSuffix(fields[2],"."),64)
  if linkErr!=nil||levelErr!=nil{continue}
  quality:=int(math.Round(link/70*100))
  if quality<0{quality=0}else if quality>100{quality=100}
  return WiFiStatus{Connected:true,Interface:name,SignalDBM:int(math.Round(level)),QualityPercent:quality}
 }
 return WiFiStatus{}
}
