package app

import (
	"crypto/ed25519"
	"fmt"

	"github.com/DragonBlack18/DeployCraft/src/trust"
)

// ensureTrustedCatalog validates authenticity INSIDE the motor before any
// installation. A caller-provided boolean cannot assert a valid signature.
func (e *InstallEngine) ensureTrustedCatalog(labUnsigned bool) error {
	if len(e.TrustedCatalogPublicKey) > 0 {
		if len(e.TrustedCatalogPublicKey) != ed25519.PublicKeySize {
			return &PreflightError{Code: "E-VER-004", Reason: "chave pública compilada inválida"}
		}
		if err := trust.Verify(e.KitRoot, e.TrustedCatalogPublicKey); err != nil {
			return &PreflightError{Code: "E-VER-004", Reason: err.Error()}
		}
		return nil
	}
	if labUnsigned {
		return nil
	}
	return &PreflightError{Code: "E-VER-004", Reason: fmt.Sprintf("motor sem chave pública; execução somente LAB ou build com chave assinada")}
}
