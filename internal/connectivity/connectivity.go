// Package connectivity provides narrow adapters for the host Wi-Fi and Bluetooth tools.
package connectivity

import (
 "context"
 "errors"
 "fmt"
 "os/exec"
 "regexp"
 "sort"
 "strconv"
 "strings"
 "time"
)

type Runner interface {
 LookPath(name string)(string,error)
 Run(ctx context.Context,input,name string,args ...string)(string,error)
}

type ExecRunner struct{}

func(ExecRunner)LookPath(name string)(string,error){return exec.LookPath(name)}
func(ExecRunner)Run(ctx context.Context,input,name string,args ...string)(string,error){
 cmd:=exec.CommandContext(ctx,name,args...)
 if input!=""{cmd.Stdin=strings.NewReader(input)}
 output,err:=cmd.CombinedOutput()
 if err!=nil{return "",fmt.Errorf("%s: %w: %s",name,err,strings.TrimSpace(string(output)))}
 return string(output),nil
}

type Manager struct{
 Runner Runner
 WiFiInterface string
}

func New()*Manager{return &Manager{Runner:ExecRunner{},WiFiInterface:"wlan0"}}

type WiFiNetwork struct{
 SSID string `json:"ssid"`
 SignalPercent int `json:"signal_percent"`
 Security string `json:"security,omitempty"`
 Connected bool `json:"connected"`
}

type WiFiConnectRequest struct{
 SSID string `json:"ssid"`
 Password string `json:"password,omitempty"`
}

func(m *Manager)runner()Runner{if m!=nil&&m.Runner!=nil{return m.Runner};return ExecRunner{}}
func(m *Manager)wifiInterface()string{if m!=nil&&strings.TrimSpace(m.WiFiInterface)!=""{return m.WiFiInterface};return "wlan0"}

func(m *Manager)ScanWiFi(ctx context.Context)([]WiFiNetwork,error){
 r:=m.runner();available:=false;successful:=false;scanErrors:=[]error{}
 if _,err:=r.LookPath("nmcli");err==nil{
  available=true
  scanCtx,cancel:=context.WithTimeout(ctx,20*time.Second);defer cancel()
  output,err:=r.Run(scanCtx,"","nmcli","-t","--escape","yes","-f","IN-USE,SIGNAL,SECURITY,SSID","device","wifi","list","--rescan","yes","ifname",m.wifiInterface())
  if err==nil{successful=true;if networks:=parseNMCLI(output);len(networks)>0{return networks,nil}}else{scanErrors=append(scanErrors,err)}
 }
 if _,err:=r.LookPath("wpa_cli");err==nil{
  available=true
  scanCtx,cancel:=context.WithTimeout(ctx,20*time.Second);defer cancel()
  output,err:=r.Run(scanCtx,"","wpa_cli","-i",m.wifiInterface(),"scan")
  if err!=nil{scanErrors=append(scanErrors,err)}else if strings.Contains(strings.ToUpper(output),"FAIL"){scanErrors=append(scanErrors,fmt.Errorf("wpa_cli rejected scan: %s",strings.TrimSpace(output)))}else{
   successful=true;deadline:=time.Now().Add(9*time.Second)
   for{
    output,err=r.Run(scanCtx,"","wpa_cli","-i",m.wifiInterface(),"scan_results")
    if err!=nil{scanErrors=append(scanErrors,err);break}
    if networks:=parseWPAScan(output);len(networks)>0{return networks,nil}
    if time.Now().After(deadline){break}
    timer:=time.NewTimer(500*time.Millisecond)
    select{case <-scanCtx.Done():timer.Stop();scanErrors=append(scanErrors,scanCtx.Err());break;case <-timer.C:}
    if scanCtx.Err()!=nil{break}
   }
  }
 }
 if len(scanErrors)>0{return nil,fmt.Errorf("Wi-Fi scan failed: %w",errors.Join(scanErrors...))}
 if successful{return []WiFiNetwork{},nil}
 if !available{return nil,errors.New("no supported Wi-Fi manager found (nmcli or wpa_cli)")}
 return []WiFiNetwork{},nil
}

func(m *Manager)ConnectWiFi(ctx context.Context,request WiFiConnectRequest)error{
 request.SSID=strings.TrimSpace(request.SSID)
 if request.SSID==""||len([]byte(request.SSID))>32{return errors.New("SSID must contain 1..32 bytes")}
 if len(request.Password)>63{return errors.New("Wi-Fi password must not exceed 63 characters")}
 if request.Password!=""&&len(request.Password)<8{return errors.New("secured Wi-Fi passwords must contain at least 8 characters")}
 r:=m.runner()
 connectCtx,cancel:=context.WithTimeout(ctx,45*time.Second);defer cancel();connectErrors:=[]error{}
 if _,err:=r.LookPath("nmcli");err==nil{
  args:=[]string{"--wait","35","device","wifi","connect",request.SSID,"ifname",m.wifiInterface()}
  input:="";if request.Password!=""{args=append([]string{"--ask"},args...);input=request.Password+"\n"}
  if _,err=r.Run(connectCtx,input,"nmcli",args...);err==nil{return nil};connectErrors=append(connectErrors,err)
 }
 if _,err:=r.LookPath("wpa_cli");err==nil{if err=m.connectWPA(connectCtx,request);err==nil{return nil};connectErrors=append(connectErrors,err)}
 if len(connectErrors)>0{return fmt.Errorf("Wi-Fi connection failed: %w",errors.Join(connectErrors...))}
 return errors.New("no supported Wi-Fi manager found (nmcli or wpa_cli)")
}

func(m *Manager)connectWPA(ctx context.Context,request WiFiConnectRequest)error{
 r:=m.runner();iface:=m.wifiInterface()
 output,err:=r.Run(ctx,"","wpa_cli","-i",iface,"add_network");if err!=nil{return err}
 id:=strings.TrimSpace(output);if _,err=strconv.Atoi(id);err!=nil{return fmt.Errorf("wpa_cli returned invalid network id")}
 cleanup:=func(){cleanupCtx,cancel:=context.WithTimeout(context.Background(),5*time.Second);defer cancel();_,_=r.Run(cleanupCtx,"","wpa_cli","-i",iface,"remove_network",id)}
 commands:=[]string{fmt.Sprintf("set_network %s ssid %s",id,strconv.Quote(request.SSID))}
 if request.Password==""{commands=append(commands,fmt.Sprintf("set_network %s key_mgmt NONE",id))}else{commands=append(commands,fmt.Sprintf("set_network %s psk %s",id,strconv.Quote(request.Password)))}
 commands=append(commands,fmt.Sprintf("enable_network %s",id),fmt.Sprintf("select_network %s",id),"save_config","quit")
 output,err=r.Run(ctx,strings.Join(commands,"\n")+"\n","wpa_cli","-i",iface)
 if err!=nil{cleanup();return errors.New("wpa_supplicant could not apply the Wi-Fi configuration")}
 if strings.Contains(output,"FAIL"){cleanup();return errors.New("wpa_supplicant rejected the Wi-Fi configuration")}
 return nil
}

func splitEscaped(line string,separator rune)[]string{
 fields:=[]string{};var b strings.Builder;escaped:=false
 for _,r:=range line{
  if escaped{b.WriteRune(r);escaped=false;continue}
  if r=='\\'{escaped=true;continue}
  if r==separator{fields=append(fields,b.String());b.Reset();continue}
  b.WriteRune(r)
 }
 fields=append(fields,b.String());return fields
}

func parseNMCLI(output string)[]WiFiNetwork{
 networks:=[]WiFiNetwork{}
 for _,line:=range strings.Split(output,"\n"){
  if strings.TrimSpace(line)==""{continue};fields:=splitEscaped(line,':');if len(fields)<4{continue}
  signal,_:=strconv.Atoi(fields[1]);ssid:=strings.TrimSpace(strings.Join(fields[3:],":"));if ssid==""{continue}
  networks=append(networks,WiFiNetwork{SSID:ssid,SignalPercent:clamp(signal),Security:strings.TrimSpace(fields[2]),Connected:strings.TrimSpace(fields[0])=="*"})
 }
 return deduplicateWiFi(networks)
}

func parseWPAScan(output string)[]WiFiNetwork{
 networks:=[]WiFiNetwork{}
 for _,line:=range strings.Split(output,"\n"){
  fields:=strings.Split(line,"\t");if len(fields)<5||strings.EqualFold(fields[0],"bssid / frequency / signal level / flags / ssid"){continue}
  dbm,err:=strconv.Atoi(strings.TrimSpace(fields[2]));if err!=nil{continue};ssid:=strings.TrimSpace(strings.Join(fields[4:],"\t"));if ssid==""{continue}
  flags:=strings.TrimSpace(fields[3]);security:="Open";if strings.Contains(flags,"WPA"){security="WPA/WPA2"}else if strings.Contains(flags,"WEP"){security="WEP"}
  networks=append(networks,WiFiNetwork{SSID:ssid,SignalPercent:clamp(2*(dbm+100)),Security:security,Connected:strings.Contains(flags,"[CURRENT]")})
 }
 return deduplicateWiFi(networks)
}

func deduplicateWiFi(input []WiFiNetwork)[]WiFiNetwork{
 bySSID:=map[string]WiFiNetwork{}
 for _,network:=range input{current,ok:=bySSID[network.SSID];if !ok||network.Connected||network.SignalPercent>current.SignalPercent{bySSID[network.SSID]=network}}
 out:=make([]WiFiNetwork,0,len(bySSID));for _,network:=range bySSID{out=append(out,network)}
 sort.Slice(out,func(i,j int)bool{if out[i].Connected!=out[j].Connected{return out[i].Connected};if out[i].SignalPercent!=out[j].SignalPercent{return out[i].SignalPercent>out[j].SignalPercent};return strings.ToLower(out[i].SSID)<strings.ToLower(out[j].SSID)})
 return out
}

func clamp(value int)int{if value<0{return 0};if value>100{return 100};return value}

type BluetoothDevice struct{
 Address string `json:"address"`
 Name string `json:"name"`
 Paired bool `json:"paired"`
 Trusted bool `json:"trusted"`
 Connected bool `json:"connected"`
}

var bluetoothAddress=regexp.MustCompile(`(?i)^[0-9a-f]{2}(:[0-9a-f]{2}){5}$`)

func(m *Manager)SetBluetoothPower(ctx context.Context,enabled bool)error{
 r:=m.runner();if _,err:=r.LookPath("bluetoothctl");err!=nil{return errors.New("bluetoothctl is not installed")}
 value:="off";if enabled{value="on"}
 runCtx,cancel:=context.WithTimeout(ctx,10*time.Second);defer cancel()
 output,err:=r.Run(runCtx,"","bluetoothctl","power",value);if err!=nil{return err}
 if strings.Contains(strings.ToLower(output),"failed"){return errors.New(strings.TrimSpace(output))}
 return nil
}

func(m *Manager)ScanBluetooth(ctx context.Context)([]BluetoothDevice,error){
 r:=m.runner();if _,err:=r.LookPath("bluetoothctl");err!=nil{return nil,errors.New("bluetoothctl is not installed")}
 scanCtx,cancel:=context.WithTimeout(ctx,18*time.Second);defer cancel()
 _,_ = r.Run(scanCtx,"","bluetoothctl","--timeout","8","scan","on")
 output,err:=r.Run(scanCtx,"","bluetoothctl","devices");if err!=nil{return nil,err}
 devices:=[]BluetoothDevice{}
 for _,line:=range strings.Split(output,"\n"){
  fields:=strings.Fields(line);if len(fields)<3||fields[0]!="Device"||!bluetoothAddress.MatchString(fields[1]){continue}
  device:=BluetoothDevice{Address:strings.ToUpper(fields[1]),Name:strings.Join(fields[2:]," ")}
  info,infoErr:=r.Run(scanCtx,"","bluetoothctl","info",device.Address);if infoErr==nil{
   device.Paired=parseBluetoothBool(info,"Paired");device.Trusted=parseBluetoothBool(info,"Trusted");device.Connected=parseBluetoothBool(info,"Connected")
  }
  devices=append(devices,device)
 }
 sort.Slice(devices,func(i,j int)bool{if devices[i].Connected!=devices[j].Connected{return devices[i].Connected};if devices[i].Paired!=devices[j].Paired{return devices[i].Paired};return strings.ToLower(devices[i].Name)<strings.ToLower(devices[j].Name)})
 return devices,nil
}

func parseBluetoothBool(info,key string)bool{
 prefix:=strings.ToLower(key)+":"
 for _,line:=range strings.Split(info,"\n"){line=strings.TrimSpace(line);if strings.HasPrefix(strings.ToLower(line),prefix){return strings.EqualFold(strings.TrimSpace(strings.TrimPrefix(line,key+":")),"yes")}}
 return false
}

func(m *Manager)BluetoothCommand(ctx context.Context,action,address string)error{
 address=strings.ToUpper(strings.TrimSpace(address));if !bluetoothAddress.MatchString(address){return errors.New("invalid Bluetooth address")}
 allowed:=map[string]bool{"pair":true,"connect":true,"disconnect":true,"remove":true};if !allowed[action]{return errors.New("unsupported Bluetooth action")}
 r:=m.runner();if _,err:=r.LookPath("bluetoothctl");err!=nil{return errors.New("bluetoothctl is not installed")}
 timeout:=15*time.Second;if action=="pair"{timeout=40*time.Second}
 runCtx,cancel:=context.WithTimeout(ctx,timeout);defer cancel()
 if action=="pair"{
  output,err:=r.Run(runCtx,"","bluetoothctl","--timeout","30","pair",address);if err!=nil{return err};if strings.Contains(strings.ToLower(output),"failed"){return errors.New(strings.TrimSpace(output))}
  _,_=r.Run(runCtx,"","bluetoothctl","trust",address)
  _,err=r.Run(runCtx,"","bluetoothctl","connect",address);return err
 }
 _,err:=r.Run(runCtx,"","bluetoothctl",action,address);return err
}
