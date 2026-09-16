package main

import (
 "os"
 "os/exec"
 "path/filepath"
 "testing"
)
func TestARM64Build(t *testing.T){
 if testing.Short(){t.Skip("cross build skipped in short mode")}
 cmd:=exec.Command("go","build","-buildvcs=false","-o",filepath.Join(t.TempDir(),"mupibox-arm64"),".")
 cmd.Env=append(os.Environ(),"GOOS=linux","GOARCH=arm64","CGO_ENABLED=0")
 if out,err:=cmd.CombinedOutput();err!=nil{t.Fatalf("ARM64 build: %v\n%s",err,out)}
}
