package library

import (
 "io/fs"
 "os"
 "path/filepath"
 "sort"
 "strings"
)

type Directory struct {
 Path string `json:"path"`
 Name string `json:"name"`
}

// Directories returns selectable folders below the configured media root.
// Hidden folders and symlinks are excluded so the admin browser cannot escape it.
func Directories(root string)([]Directory,error){
 abs,err:=filepath.Abs(root);if err!=nil{return nil,err}
 abs,err=filepath.EvalSymlinks(abs);if err!=nil{return nil,err}
 out:=[]Directory{}
 err=filepath.WalkDir(abs,func(current string,entry fs.DirEntry,walkErr error)error{
  if walkErr!=nil{return walkErr}
  if current==abs{return nil}
  if strings.HasPrefix(entry.Name(),"."){if entry.IsDir(){return filepath.SkipDir};return nil}
  if entry.Type()&os.ModeSymlink!=0{if entry.IsDir(){return filepath.SkipDir};return nil}
  if !entry.IsDir(){return nil}
  relative,err:=filepath.Rel(abs,current);if err!=nil{return err}
  relative=filepath.ToSlash(relative)
  out=append(out,Directory{Path:relative,Name:entry.Name()})
  return nil
 })
 if err!=nil{return nil,err}
 sort.Slice(out,func(i,j int)bool{return naturalLess(out[i].Path,out[j].Path)})
 return out,nil
}
