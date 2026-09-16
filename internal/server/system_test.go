package server

import (
 "encoding/json"
 "net/http/httptest"
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

func TestSystemStatusEndpoint(t *testing.T){
 api:=&API{System:func()SystemStatus{return SystemStatus{WiFi:WiFiStatus{Connected:true,Interface:"wlan0",SignalDBM:-55,QualityPercent:80}}}}
 response:=httptest.NewRecorder()
 api.Handler().ServeHTTP(response,httptest.NewRequest("GET","/api/system",nil))
 if response.Code!=200{t.Fatalf("status=%d body=%s",response.Code,response.Body.String())}
 var got SystemStatus
 if err:=json.Unmarshal(response.Body.Bytes(),&got);err!=nil{t.Fatal(err)}
 if !got.WiFi.Connected||got.WiFi.QualityPercent!=80{t.Fatalf("unexpected response: %#v",got)}
}
