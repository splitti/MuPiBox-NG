package server

import (
 "net/http"
 "net/http/httptest"
 "strings"
 "testing"

 "mupibox/internal/store"
)

func TestAdminPasswordProtectsAdminAPI(t *testing.T){
 db,err:=store.Open(":memory:");if err!=nil{t.Fatal(err)};defer db.Close()
 defaults:=store.BoxSettings{Language:"de",AdminLanguage:"de",Audio:store.AudioSettings{StartupVolume:30,MaxVolume:60},Display:store.DisplaySettings{Brightness:100,UISize:"normal"},Theme:"modern-dark"}
 if _,err=db.EnsureBoxSettings(defaults);err!=nil{t.Fatal(err)}
 handler:=(&API{Store:db}).Handler()
 call:=func(method,path,body string,cookie *http.Cookie)*httptest.ResponseRecorder{request:=httptest.NewRequest(method,path,strings.NewReader(body));if body!=""{request.Header.Set("Content-Type","application/json")};if cookie!=nil{request.AddCookie(cookie)};response:=httptest.NewRecorder();handler.ServeHTTP(response,request);return response}
 if response:=call(http.MethodGet,"/api/admin/settings","",nil);response.Code!=200{t.Fatalf("unprotected setup status=%d",response.Code)}
 response:=call(http.MethodPut,"/api/admin/password",`{"current_password":"","new_password":"a-safe-admin-password"}`,nil);if response.Code!=200{t.Fatalf("set password status=%d body=%s",response.Code,response.Body.String())}
 encoded,ok,err:=db.LoadAdminPasswordHash();if err!=nil||!ok||strings.Contains(encoded,"a-safe-admin-password")||!verifyAdminPassword(encoded,"a-safe-admin-password"){t.Fatalf("password was not securely persisted: ok=%v err=%v value=%q",ok,err,encoded)}
 if response=call(http.MethodGet,"/api/admin/settings","",nil);response.Code!=http.StatusUnauthorized{t.Fatalf("admin without session status=%d",response.Code)}
 if response=call(http.MethodPost,"/api/admin/login",`{"password":"wrong-password"}`,nil);response.Code!=http.StatusUnauthorized{t.Fatalf("wrong login status=%d",response.Code)}
 response=call(http.MethodPost,"/api/admin/login",`{"password":"a-safe-admin-password"}`,nil);if response.Code!=200{t.Fatalf("login status=%d body=%s",response.Code,response.Body.String())}
 var session *http.Cookie;for _,cookie:=range response.Result().Cookies(){if cookie.Name==adminSessionCookie&&cookie.MaxAge>0{session=cookie}}
 if session==nil{t.Fatal("login did not return an admin session cookie")}
 if response=call(http.MethodGet,"/api/admin/settings","",session);response.Code!=200{t.Fatalf("authenticated admin status=%d",response.Code)}
}

func TestPasswordHashRejectsInvalidPassword(t *testing.T){encoded,err:=hashAdminPassword("1234567890");if err!=nil{t.Fatal(err)};if verifyAdminPassword(encoded,"1234567891"){t.Fatal("wrong password accepted")};if _,err=hashAdminPassword("short");err==nil{t.Fatal("short password accepted")}}
