package connectivity

import (
 "context"
 "errors"
 "strings"
 "testing"
)

type fakeRunner struct{
 paths map[string]bool
 outputs map[string]string
 errors map[string]error
 calls []string
 inputs []string
}

func(f *fakeRunner)LookPath(name string)(string,error){if f.paths[name]{return "/usr/bin/"+name,nil};return "",errors.New("missing")}
func(f *fakeRunner)Run(_ context.Context,input,name string,args ...string)(string,error){
 key:=name+" "+strings.Join(args," ");f.calls=append(f.calls,key);f.inputs=append(f.inputs,input)
 if err,ok:=f.errors[key];ok{return "",err}
 if output,ok:=f.outputs[key];ok{return output,nil}
 return "",nil
}

func TestParseWiFiScans(t *testing.T){
 nm:=parseNMCLI("*:79:WPA2:Home\n:35:WPA2:Guest\\:West\n:62:WPA2:Home\n")
 if len(nm)!=2||!nm[0].Connected||nm[0].SSID!="Home"||nm[1].SSID!="Guest:West"{t.Fatalf("unexpected nmcli scan: %#v",nm)}
 wpa:=parseWPAScan("bssid / frequency / signal level / flags / ssid\naa:bb:cc:dd:ee:ff\t2412\t-61\t[WPA2-PSK-CCMP][ESS]\thocuspocus\n")
 if len(wpa)!=1||wpa[0].SSID!="hocuspocus"||wpa[0].SignalPercent!=78||wpa[0].Security!="WPA/WPA2"{t.Fatalf("unexpected wpa scan: %#v",wpa)}
}

func TestScanWiFiFallsBackFromEmptyNMCLIToWPA(t *testing.T){
 runner:=&fakeRunner{paths:map[string]bool{"nmcli":true,"wpa_cli":true},outputs:map[string]string{
  "nmcli -t --escape yes -f IN-USE,SIGNAL,SECURITY,SSID device wifi list --rescan yes ifname wlan0":"",
  "wpa_cli -i wlan0 scan":"OK\n",
  "wpa_cli -i wlan0 scan_results":"bssid / frequency / signal level / flags / ssid\naa:bb:cc:dd:ee:ff\t2412\t-55\t[WPA2-PSK-CCMP][ESS]\thocuspocus\n",
 }}
 networks,err:=(&Manager{Runner:runner,WiFiInterface:"wlan0"}).ScanWiFi(context.Background())
 if err!=nil{t.Fatal(err)}
 if len(networks)!=1||networks[0].SSID!="hocuspocus"{t.Fatalf("unexpected networks: %#v",networks)}
 if !strings.Contains(strings.Join(runner.calls,"\n"),"wpa_cli -i wlan0 scan_results"){t.Fatalf("wpa_cli fallback was not used: %#v",runner.calls)}
}

func TestConnectWiFiUsesArgumentSafeNMCLI(t *testing.T){
 runner:=&fakeRunner{paths:map[string]bool{"nmcli":true},outputs:map[string]string{}}
 manager:=&Manager{Runner:runner,WiFiInterface:"wlan0"}
 if err:=manager.ConnectWiFi(context.Background(),WiFiConnectRequest{SSID:"Kids; network",Password:"safe password"});err!=nil{t.Fatal(err)}
 call:=runner.calls[len(runner.calls)-1]
 if !strings.Contains(call,"nmcli --ask --wait 35 device wifi connect Kids; network ifname wlan0"){t.Fatalf("unexpected command: %s",call)}
 if runner.inputs[len(runner.inputs)-1]!="safe password\n"{t.Fatal("password was not supplied through stdin")}
 if err:=manager.ConnectWiFi(context.Background(),WiFiConnectRequest{SSID:"x",Password:"short"});err==nil{t.Fatal("short secured password accepted")}
}

func TestConnectWiFiFallsBackFromNMCLIToWPA(t *testing.T){
 nmCall:="nmcli --ask --wait 35 device wifi connect Home ifname wlan0"
 runner:=&fakeRunner{paths:map[string]bool{"nmcli":true,"wpa_cli":true},errors:map[string]error{nmCall:errors.New("device is not managed")},outputs:map[string]string{"wpa_cli -i wlan0 add_network":"4\n"}}
 manager:=&Manager{Runner:runner,WiFiInterface:"wlan0"}
 if err:=manager.ConnectWiFi(context.Background(),WiFiConnectRequest{SSID:"Home",Password:"safe password"});err!=nil{t.Fatal(err)}
 if !strings.Contains(strings.Join(runner.calls,"\n"),"wpa_cli -i wlan0 add_network"){t.Fatalf("wpa_cli fallback was not used: %#v",runner.calls)}
}

func TestBluetoothScanAndCommandValidation(t *testing.T){
 runner:=&fakeRunner{paths:map[string]bool{"bluetoothctl":true},outputs:map[string]string{
  "bluetoothctl devices":"Device AA:BB:CC:DD:EE:FF Kids Headphones\n",
  "bluetoothctl info AA:BB:CC:DD:EE:FF":"Name: Kids Headphones\nPaired: yes\nTrusted: yes\nConnected: no\n",
 }}
 manager:=&Manager{Runner:runner}
 devices,err:=manager.ScanBluetooth(context.Background());if err!=nil{t.Fatal(err)}
 if len(devices)!=1||!devices[0].Paired||!devices[0].Trusted||devices[0].Connected{t.Fatalf("unexpected devices: %#v",devices)}
 if err=manager.BluetoothCommand(context.Background(),"connect","not-a-mac");err==nil{t.Fatal("invalid address accepted")}
}
