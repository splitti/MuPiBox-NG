package server

import (
 "crypto/hmac"
 "crypto/rand"
 "crypto/sha256"
 "crypto/subtle"
 "encoding/base64"
 "errors"
 "fmt"
 "net"
 "net/http"
 "strconv"
 "strings"
 "sync"
 "time"
 "unicode/utf8"
)

const adminSessionCookie="mupibox_admin_session"
const passwordIterations=210000

type adminSessionStore struct{
 mu sync.Mutex
 tokens map[string]time.Time
 attempts map[string]adminLoginAttempt
}
type adminLoginAttempt struct{Failures int;BlockedUntil time.Time}

func derivePasswordKey(password,salt []byte,iterations,length int)[]byte{
 result:=make([]byte,0,length)
 for block:=1;len(result)<length;block++{
  mac:=hmac.New(sha256.New,password);mac.Write(salt);mac.Write([]byte{byte(block>>24),byte(block>>16),byte(block>>8),byte(block)})
  sum:=mac.Sum(nil);value:=append([]byte(nil),sum...)
  for round:=1;round<iterations;round++{mac=hmac.New(sha256.New,password);mac.Write(sum);sum=mac.Sum(nil);for i:=range value{value[i]^=sum[i]}}
  result=append(result,value...)
 }
 return result[:length]
}

func hashAdminPassword(password string)(string,error){
 if utf8.RuneCountInString(password)<10{return "",errors.New("password must contain at least 10 characters")}
 if len([]byte(password))>128{return "",errors.New("password must not exceed 128 bytes")}
 salt:=make([]byte,16);if _,err:=rand.Read(salt);err!=nil{return "",err}
 key:=derivePasswordKey([]byte(password),salt,passwordIterations,32)
 return fmt.Sprintf("pbkdf2-sha256$%d$%s$%s",passwordIterations,base64.RawStdEncoding.EncodeToString(salt),base64.RawStdEncoding.EncodeToString(key)),nil
}

func verifyAdminPassword(encoded,password string)bool{
 parts:=strings.Split(encoded,"$");if len(parts)!=4||parts[0]!="pbkdf2-sha256"{return false}
 iterations,err:=strconv.Atoi(parts[1]);if err!=nil||iterations<100000||iterations>1000000{return false}
 salt,err:=base64.RawStdEncoding.DecodeString(parts[2]);if err!=nil||len(salt)<16{return false}
 expected,err:=base64.RawStdEncoding.DecodeString(parts[3]);if err!=nil||len(expected)!=32{return false}
 actual:=derivePasswordKey([]byte(password),salt,iterations,len(expected))
 return subtle.ConstantTimeCompare(actual,expected)==1
}

func(a *API)sessionStore()*adminSessionStore{
 a.adminSessionInit.Do(func(){a.adminSessions=&adminSessionStore{tokens:map[string]time.Time{},attempts:map[string]adminLoginAttempt{}}})
 return a.adminSessions
}

func(a *API)newAdminSession(w http.ResponseWriter,r *http.Request)error{
 raw:=make([]byte,32);if _,err:=rand.Read(raw);err!=nil{return err};token:=base64.RawURLEncoding.EncodeToString(raw);expires:=time.Now().Add(12*time.Hour)
 sessions:=a.sessionStore();sessions.mu.Lock();sessions.tokens[token]=expires;for value,until:=range sessions.tokens{if time.Now().After(until){delete(sessions.tokens,value)}};sessions.mu.Unlock()
 http.SetCookie(w,&http.Cookie{Name:adminSessionCookie,Value:token,Path:"/",Expires:expires,MaxAge:12*60*60,HttpOnly:true,Secure:r.TLS!=nil,SameSite:http.SameSiteStrictMode})
 return nil
}

func(a *API)clearAdminSessions(w http.ResponseWriter,r *http.Request){sessions:=a.sessionStore();sessions.mu.Lock();sessions.tokens=map[string]time.Time{};sessions.mu.Unlock();http.SetCookie(w,&http.Cookie{Name:adminSessionCookie,Value:"",Path:"/",MaxAge:-1,HttpOnly:true,Secure:r.TLS!=nil,SameSite:http.SameSiteStrictMode})}

func(a *API)adminAuthenticated(r *http.Request)bool{
 cookie,err:=r.Cookie(adminSessionCookie);if err!=nil{return false};sessions:=a.sessionStore();sessions.mu.Lock();defer sessions.mu.Unlock();expires,ok:=sessions.tokens[cookie.Value];if !ok||time.Now().After(expires){delete(sessions.tokens,cookie.Value);return false};return true
}

func(a *API)adminPasswordHash()(string,bool,error){if a.Store==nil{return "",false,nil};return a.Store.LoadAdminPasswordHash()}

func requestIsLoopback(r *http.Request)bool{host,_,err:=net.SplitHostPort(r.RemoteAddr);if err!=nil{host=r.RemoteAddr};ip:=net.ParseIP(strings.Trim(host,"[]"));return ip!=nil&&ip.IsLoopback()}

func requestHost(r *http.Request)string{host,_,err:=net.SplitHostPort(r.RemoteAddr);if err!=nil{return r.RemoteAddr};return host}

func(a *API)loginAllowed(r *http.Request)bool{sessions:=a.sessionStore();sessions.mu.Lock();defer sessions.mu.Unlock();attempt:=sessions.attempts[requestHost(r)];return attempt.BlockedUntil.IsZero()||time.Now().After(attempt.BlockedUntil)}
func(a *API)recordLogin(r *http.Request,success bool){sessions:=a.sessionStore();sessions.mu.Lock();defer sessions.mu.Unlock();host:=requestHost(r);if success{delete(sessions.attempts,host);return};attempt:=sessions.attempts[host];attempt.Failures++;if attempt.Failures>=5{attempt.Failures=0;attempt.BlockedUntil=time.Now().Add(5*time.Minute)};sessions.attempts[host]=attempt}

func(a *API)requiresAdminAuthentication(r *http.Request)bool{
 if r.URL.Path=="/api/admin/auth"||r.URL.Path=="/api/admin/login"{return false}
 if strings.HasPrefix(r.URL.Path,"/api/admin/"){return true}
 if strings.HasPrefix(r.URL.Path,"/api/connectivity/")&&!requestIsLoopback(r){return true}
 return false
}

func(a *API)authorizeRequest(w http.ResponseWriter,r *http.Request)bool{
 if !a.requiresAdminAuthentication(r){return true}
 _,protected,err:=a.adminPasswordHash();if err!=nil{problem(w,500,err);return false}
 if !protected||a.adminAuthenticated(r){return true}
 problem(w,http.StatusUnauthorized,errors.New("admin authentication required"));return false
}
