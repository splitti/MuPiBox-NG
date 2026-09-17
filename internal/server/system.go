package server

import (
 "bufio"
 "io"
 "math"
 "net"
 "os"
 "path/filepath"
 "strconv"
 "strings"
)

type WiFiStatus struct {
 Connected bool `json:"connected"`
 Interface string `json:"interface,omitempty"`
 SignalDBM int `json:"signal_dbm,omitempty"`
 QualityPercent int `json:"quality_percent,omitempty"`
}

type BatteryStatus struct {
 Available bool `json:"available"`
 Percent int `json:"percent,omitempty"`
 Charging bool `json:"charging,omitempty"`
}

type SystemStatus struct {
 Online bool `json:"online"`
 WiFi WiFiStatus `json:"wifi"`
 Battery BatteryStatus `json:"battery"`
}

func (a *API) currentSystemStatus() SystemStatus {
 if a.System!=nil{return a.System()}
 return readSystemStatus()
}

func readSystemStatus() SystemStatus {
 status:=SystemStatus{Online:networkAvailable(),Battery:readBatteryStatus("/sys/class/power_supply")}
 file,err:=os.Open("/proc/net/wireless")
 if err!=nil{return status}
 defer file.Close()
 status.WiFi=parseWiFiStatus(file)
 return status
}

func networkAvailable()bool{
 interfaces,err:=net.Interfaces();if err!=nil{return false}
 for _,iface:=range interfaces{
  if iface.Flags&net.FlagUp==0||iface.Flags&net.FlagRunning==0||iface.Flags&net.FlagLoopback!=0{continue}
  addresses,err:=iface.Addrs();if err==nil&&len(addresses)>0{return true}
 }
 return false
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

func readBatteryStatus(root string) BatteryStatus {
 entries,err:=os.ReadDir(root)
 if err!=nil{return BatteryStatus{}}
 for _,entry:=range entries{
  if !entry.IsDir(){continue}
  dir:=filepath.Join(root,entry.Name())
  kind,err:=os.ReadFile(filepath.Join(dir,"type"))
  if err!=nil||strings.TrimSpace(string(kind))!="Battery"{continue}
  rawCapacity,err:=os.ReadFile(filepath.Join(dir,"capacity"))
  if err!=nil{continue}
  percent,err:=strconv.Atoi(strings.TrimSpace(string(rawCapacity)))
  if err!=nil{continue}
  if percent<0{percent=0}else if percent>100{percent=100}
  charging:=false
  if rawStatus,readErr:=os.ReadFile(filepath.Join(dir,"status"));readErr==nil{
   charging=strings.EqualFold(strings.TrimSpace(string(rawStatus)),"Charging")
  }
  return BatteryStatus{Available:true,Percent:percent,Charging:charging}
 }
 return BatteryStatus{}
}
