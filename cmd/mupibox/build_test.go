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
 font,err:=os.Stat(filepath.Join("..","..","ui","qtquick","assets","PressStart2P-Regular.ttf"));if err!=nil{t.Fatal(err)}
 if font.Size()<10000{t.Fatalf("bundled retro font is unexpectedly small: %d bytes",font.Size())}
 text:=string(source)
 if !strings.Contains(text,"import QtQuick.VirtualKeyboard")||!strings.Contains(text,"InputPanel {"){t.Fatal("native Qt virtual keyboard is not wired into the touchscreen UI")}
 if strings.Contains(text,"wifiKeyboardRows")||strings.Contains(text,"adminHintVisible"){t.Fatal("legacy touchscreen keyboard or device admin overlay is still present")}
 service,err:=os.ReadFile(filepath.Join("..","..","deploy","mupibox-ui.service"));if err!=nil{t.Fatal(err)}
 if !strings.Contains(string(service),"QT_IM_MODULE=qtvirtualkeyboard"){t.Fatal("Qt virtual keyboard input method is not enabled in the UI service")}
 installer,err:=os.ReadFile(filepath.Join("..","..","scripts","install-dietpi.sh"));if err!=nil{t.Fatal(err)}
 if !strings.Contains(string(installer),"qml6-module-qtquick-virtualkeyboard"){t.Fatal("Qt virtual keyboard package is missing from the DietPi installer")}
}
