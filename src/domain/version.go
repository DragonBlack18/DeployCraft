package domain

import (
 "fmt"
 "math/big"
 "regexp"
 "strings"
)

var numericVersion = regexp.MustCompile(`^[0-9]+(\.[0-9]+)*$`)

// CompareVersions compares numeric components, padding missing segments with zero.
func CompareVersions(a,b string)(int,error){
 if !numericVersion.MatchString(a)||!numericVersion.MatchString(b){return 0,fmt.Errorf("E-CAT-003: versão inválida %q ou %q",a,b)}
 aa,bb:=strings.Split(a,"."),strings.Split(b,".")
 n:=len(aa);if len(bb)>n{n=len(bb)}
 for i:=0;i<n;i++ {
  x,y:=new(big.Int),new(big.Int)
  if i<len(aa){x.SetString(aa[i],10)}
  if i<len(bb){y.SetString(bb[i],10)}
  if c:=x.Cmp(y);c!=0{return c,nil}
 }
 return 0,nil
}
func ValidVersion(v string)bool{return numericVersion.MatchString(v)}
