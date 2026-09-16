package library

import (
 "os"
 "path/filepath"
 "testing"
)

func TestScanUsesDefaultCoverWhenArtworkIsMissing(t *testing.T) {
 root:=t.TempDir()
 dir:=filepath.Join(root,"No Artwork")
 if err:=os.Mkdir(dir,0700);err!=nil{t.Fatal(err)}
 if err:=os.WriteFile(filepath.Join(dir,"01.mp3"),[]byte("fixture"),0600);err!=nil{t.Fatal(err)}

 l,err:=Scan(root)
 if err!=nil{t.Fatal(err)}
 if len(l.Folders)!=1{t.Fatalf("folders: %+v",l.Folders)}
 if l.Folders[0].Cover!=DefaultCover{t.Fatalf("cover=%q want %q",l.Folders[0].Cover,DefaultCover)}
 if l.Folders[0].CoverPath!=""{t.Fatalf("default cover must not resolve to a media-library path: %q",l.Folders[0].CoverPath)}
}
