package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/DragonBlack18/DeployCraft/src/domain"
)

// Fingerprint identifies the exact normalized catalog contracts approved by a
// run. Map insertion order cannot change the digest. This is a consistency
// check, not an authenticated signature or substitute for trust.Verify.
func Fingerprint(c domain.Catalog) (string,error){
	type content struct {
		Index domain.Index `json:"index"`
		Packages []domain.Package `json:"packages"`
	}
	ids:=make([]string,0,len(c.Packages))
	for id:=range c.Packages{ids=append(ids,id)}
	sort.Strings(ids)
	payload:=content{Index:c.Index,Packages:make([]domain.Package,0,len(ids))}
	for _,id:=range ids {
		p:=c.Packages[id]
		if p.ID!=id {return "",fmt.Errorf("E-CAT-005: mapa e pacote discordam: %s",id)}
		payload.Packages=append(payload.Packages,p)
	}
	raw,err:=json.Marshal(payload)
	if err!=nil{return "",fmt.Errorf("E-CAT-003: catálogo não serializável: %w",err)}
	digest:=sha256.Sum256(append([]byte("DeployCraft/catalog/fingerprint/v1\x00"),raw...))
	return hex.EncodeToString(digest[:]),nil
}
