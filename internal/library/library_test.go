package library

import (
 "os"
 "path/filepath"
 "testing"
)
func TestScanAndContainment(t *testing.T){
 root:=t.TempDir();outside:=filepath.Join(t.TempDir(),"private.mp3");os.WriteFile(outside,[]byte("secret"),0600)
 dir:=filepath.Join(root,"Story");os.Mkdir(dir,0700)
 for _,n:=range []string{"10.mp3","2.mp3","01.mp3","cover.jpg","notes.txt"}{os.WriteFile(filepath.Join(dir,n),[]byte("fixture"),0600)}
 os.Symlink(outside,filepath.Join(dir,"escape.mp3"));os.Mkdir(filepath.Join(root,".private"),0700);os.WriteFile(filepath.Join(root,".private","hidden.mp3"),nil,0600)
 l,err:=Scan(root);if err!=nil{t.Fatal(err)};if len(l.Folders)!=1{t.Fatalf("folders: %+v",l.Folders)};f:=l.Folders[0]
 if len(f.Tracks)!=3||f.Tracks[0].Title!="01"||f.Tracks[1].Title!="2"||f.Tracks[2].Title!="10"{t.Fatalf("order: %+v",f.Tracks)}
 if f.Cover==""{t.Fatal("cover missing")}
 os.Remove(f.Tracks[0].Path);os.Symlink(outside,f.Tracks[0].Path)
 if l.Validate(f.Tracks[0].Path)==nil{t.Fatal("accepted escaped symlink after scan")}
 again,err:=Scan(root);if err!=nil{t.Fatal(err)};if again.Folders[0].ID!=f.ID{t.Fatal("folder identity changed")}
}
func TestEmptyAndMissing(t *testing.T){l,e:=Scan(t.TempDir());if e!=nil||l.Folders==nil||len(l.Folders)!=0{t.Fatalf("%+v %v",l,e)};if _,e=Scan(filepath.Join(t.TempDir(),"missing"));e==nil{t.Fatal("expected missing directory error")}}
