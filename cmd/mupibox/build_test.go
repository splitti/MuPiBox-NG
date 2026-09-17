package main

import (
 "os"
 "os/exec"
 "path/filepath"
 "regexp"
 "strings"
 "testing"
)

func TestARM64Build(t *testing.T){
 if testing.Short(){t.Skip("cross build skipped in short mode")}
 cmd:=exec.Command("go","build","-buildvcs=false","-o",filepath.Join(t.TempDir(),"mupibox-arm64"),".")
 cmd.Env=append(os.Environ(),"GOOS=linux","GOARCH=arm64","CGO_ENABLED=0")
 if out,err:=cmd.CombinedOutput();err!=nil{t.Fatalf("ARM64 build: %v\n%s",err,out)}
}

func TestQtQuickSourceHasNoObjectSeparators(t *testing.T){
 source,err:=os.ReadFile(filepath.Join("..","..","ui","qtquick","Main.qml"));if err!=nil{t.Fatal(err)}
 invalid:=regexp.MustCompile(`}\s*;\s*[A-Z][A-Za-z0-9_]*\s*\{`)
 if match:=invalid.Find(source);match!=nil{t.Fatalf("invalid semicolon between QML objects: %q",match)}
 if strings.Count(string(source),"{")!=strings.Count(string(source),"}"){t.Fatal("unbalanced QML braces")}
}
