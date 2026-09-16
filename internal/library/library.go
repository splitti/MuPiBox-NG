// Package library resolves local media without exposing arbitrary paths to clients.
package library

import (
 "crypto/sha256"
 "encoding/hex"
 "fmt"
 "io/fs"
 "os"
 "path/filepath"
 "sort"
 "strings"
)

type Track struct {
 ID string `json:"id"`
 Title string `json:"title"`
 Path string `json:"-"`
}
type Folder struct {
 ID string `json:"id"`
 Name string `json:"name"`
 Relative string `json:"relative"`
 Tracks []Track `json:"tracks"`
 Cover string `json:"cover,omitempty"`
 CoverPath string `json:"-"`
}
type Library struct { Root string; Folders []Folder }
func id(s string) string { h:=sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:16]) }
func Scan(root string) (*Library,error) {
 abs,err:=filepath.Abs(root); if err!=nil{return nil,err}
 abs,err=filepath.EvalSymlinks(abs); if err!=nil{return nil,err}
 info,err:=os.Stat(abs); if err!=nil{return nil,err}; if !info.IsDir(){return nil,fmt.Errorf("music path is not a directory")}
 l:=&Library{Root:abs,Folders:[]Folder{}}
 folders:=map[string]*Folder{}
 err=filepath.WalkDir(abs,func(path string,d fs.DirEntry,err error)error{
  if err!=nil{return err}
  if path!=abs && strings.HasPrefix(d.Name(),"."){if d.IsDir(){return filepath.SkipDir};return nil}
  if d.IsDir() || d.Type()&os.ModeSymlink!=0{return nil}
  info,err:=d.Info();if err!=nil{return err};if !info.Mode().IsRegular(){return nil}
  ext:=strings.ToLower(filepath.Ext(path));if !strings.Contains("|.mp3|.flac|.ogg|.opus|.wav|.m4a|.aac|", "|"+ext+"|"){return nil}
  dir:=filepath.Dir(path);rel,_:=filepath.Rel(abs,dir);rel=filepath.ToSlash(rel)
  f:=folders[rel];if f==nil{f=&Folder{ID:id(rel),Name:filepath.Base(dir),Relative:rel,Tracks:[]Track{}};folders[rel]=f}
  trackRel,_:=filepath.Rel(abs,path)
  f.Tracks=append(f.Tracks,Track{ID:id(filepath.ToSlash(trackRel)),Title:strings.TrimSuffix(d.Name(),filepath.Ext(path)),Path:path})
  return nil
 });if err!=nil{return nil,err}
 for _,f:=range folders{
  sort.Slice(f.Tracks,func(i,j int)bool{return naturalLess(f.Tracks[i].Title,f.Tracks[j].Title)})
  for _,name:=range []string{"cover.jpg","cover.png","folder.jpg","folder.png"}{
   p:=filepath.Join(abs,filepath.FromSlash(f.Relative),name);st,e:=os.Lstat(p)
   if e==nil && st.Mode().IsRegular(){f.Cover="/api/cover/"+f.ID;f.CoverPath=p;break}
  }
  l.Folders=append(l.Folders,*f)
 }
 sort.Slice(l.Folders,func(i,j int)bool{return naturalLess(l.Folders[i].Relative,l.Folders[j].Relative)})
 return l,nil
}
func (l *Library) Folder(id string)(Folder,bool){for _,f:=range l.Folders{if f.ID==id{return f,true}};return Folder{},false}
// Resolve again at use time: a symlink introduced after scanning cannot escape the root.
func (l *Library) Validate(path string) error {
 real,err:=filepath.EvalSymlinks(path);if err!=nil{return err}
 rel,err:=filepath.Rel(l.Root,real);if err!=nil{return err}
 if rel==".." || strings.HasPrefix(rel,".."+string(filepath.Separator)){return fmt.Errorf("media outside music directory")}
 st,err:=os.Stat(real);if err!=nil{return err};if !st.Mode().IsRegular(){return fmt.Errorf("not a regular file")};return nil
}
func naturalLess(a,b string)bool{
 aa,bb:=strings.ToLower(a),strings.ToLower(b)
 for i,j:=0,0;i<len(aa)&&j<len(bb);{
  if aa[i]>='0'&&aa[i]<='9'&&bb[j]>='0'&&bb[j]<='9'{
   x,y:=i,j;for i<len(aa)&&aa[i]>='0'&&aa[i]<='9'{i++};for j<len(bb)&&bb[j]>='0'&&bb[j]<='9'{j++}
   an,bn:=strings.TrimLeft(aa[x:i],"0"),strings.TrimLeft(bb[y:j],"0")
   if len(an)!=len(bn){return len(an)<len(bn)};if an!=bn{return an<bn}
  }else{if aa[i]!=bb[j]{return aa[i]<bb[j]};i++;j++}
 }
 if len(aa)!=len(bb){return len(aa)<len(bb)};return a<b
}
