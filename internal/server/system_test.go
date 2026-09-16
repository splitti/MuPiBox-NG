package server

import (
 "encoding/json"
 "net/http/httptest"
 "os"
 "path/filepath"
 "strings"
 "testing"
)

func TestParseWiFiStatus(t *testing.T){
 input:=`Inter-| sta-|   Quality        |   Discarded packets               | Missed | WE
 face | tus | link level noise |  nwid  crypt   frag  retry   misc | beacon | 22
 wlan0: 0000   49.  -61.  -256        0      0      0      0      0        0`
 got:=parseWiFiStatus(strings.NewReader(input))
 if !got.Connected||got.Interface!="wlan0"||got.SignalDBM!=-61||got.QualityPercent!=70{t.Fatalf("unexpected WiFi status: %#v",got)}
}

func TestReadBatteryStatus(t *testing.T){
 root:=t.TempDir()
 batteryDir:=filepath.Join(root,"BAT0")
 if err:=os.Mkdir(batteryDir,0700);err!=nil{t.Fatal(err)}
 for name,value:=range map[string]string{"type":"Battery\n","capacity":"74\n","status":"Charging\n"}{
  if err:=os.WriteFile(filepath.Join(batteryDir,name),[]byte(value),0600);err!=nil{t.Fatal(err)}
 }
 got:=readBatteryStatus(root)
 if !got.Available||got.Percent!=74||!got.Charging{t.Fatalf("unexpected battery status: %#v",got)}
}

func TestSystemStatusEndpoint(t *testing.T){
 api:=&API{System:func()SystemStatus{return SystemStatus{WiFi:WiFiStatus{Connected:true,Interface:"wlan0",SignalDBM:-55,QualityPercent:80},Battery:BatteryStatus{Available:true,Percent:50}}}}
 response:=httptest.NewRecorder()
 api.Handler().ServeHTTP(response,httptest.NewRequest("GET","/api/system",nil))
 if response.Code!=200{t.Fatalf("status=%d body=%s",response.Code,response.Body.String())}
 var got SystemStatus
 if err:=json.Unmarshal(response.Body.Bytes(),&got);err!=nil{t.Fatal(err)}
 if !got.WiFi.Connected||got.WiFi.QualityPercent!=80||!got.Battery.Available||got.Battery.Percent!=50{t.Fatalf("unexpected response: %#v",got)}
}
