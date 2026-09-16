package server

import (
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "os"
 "path/filepath"
 "strings"
 "testing"
 "mupibox/internal/audio"
 "mupibox/internal/core"
 "mupibox/internal/library"
)
func TestAllInputsControlOnePlayer(t *testing.T){
 dir:=t.TempDir();os.WriteFile(filepath.Join(dir,"track.wav"),nil,0600);l,_:=library.Scan(dir);p,_:=core.New(l,&audio.Simulated{},50);defer p.Close()
 a:=&API{Player:p,Library:l,Inputs:InputConfig{Simulate:true,Buttons:map[string]core.Command{"pause":{Action:"pause"}},RFID:map[string]string{"card":l.Folders[0].ID}}};h:=a.Handler()
 req:=func(path,body,origin string)int{r:=httptest.NewRequest(http.MethodPost,path,strings.NewReader(body));r.Header.Set("Content-Type","application/json");if origin!=""{r.Header.Set("Origin",origin)};w:=httptest.NewRecorder();h.ServeHTTP(w,r);return w.Code}
 if req("/api/input",`{"module":"rfid","id":"card"}`,"")!=200{t.Fatal("RFID failed")}
 if req("/api/input",`{"module":"button","id":"pause"}`,"")!=200||p.Status().State!="paused"{t.Fatal("button did not pause RFID playback")}
 if req("/api/command",`{"action":"play"}`,"")!=200||p.Status().State!="playing"{t.Fatal("browser did not resume")}
 if req("/api/command",`{"action":"pause"}`,"https://other.example")!=403{t.Fatal("cross-origin accepted")}
 if req("/api/command",`{"action":"pause","extra":1}`,"")!=400{t.Fatal("unknown field accepted")}
 if req("/api/command",`{"action":"pause"} {}`,"")!=400{t.Fatal("trailing data accepted")}
 a.Inputs.Simulate=false;if req("/api/input",`{"module":"rfid","id":"card"}`,"")!=404{t.Fatal("simulation exposed")}
 w:=httptest.NewRecorder();h.ServeHTTP(w,httptest.NewRequest("GET","/api/status",nil));var s core.Status;if err:=json.Unmarshal(w.Body.Bytes(),&s);err!=nil||s.State!="playing"{t.Fatal("shared status failed",err)}
 if strings.Contains(w.Body.String(),dir){t.Fatal("filesystem path leaked")}
}
