package catalog

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/DragonBlack18/DeployCraft/src/domain"
)

var idPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
var hashPattern = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)
var guidPattern = regexp.MustCompile(`^\{[0-9A-Fa-f]{8}(-[0-9A-Fa-f]{4}){3}-[0-9A-Fa-f]{12}\}$`)

func validID(s string) bool { return idPattern.MatchString(s) }

func validatePackage(p domain.Package) error {
	if p.Schema != 1 {
		return fmt.Errorf("E-CAT-004 %s: schema não suportado", p.ID)
	}
	if !validID(p.ID) || strings.TrimSpace(p.Name) == "" || !domain.ValidVersion(p.Version) {
		return fmt.Errorf("E-CAT-003: id, nome ou versão inválida")
	}
	if p.Platform.OS != "windows" || len(p.Platform.Arch) == 0 || p.Platform.MinBuild < 0 {
		return fmt.Errorf("E-CAT-003 %s: platform inválida", p.ID)
	}
	archSeen := map[string]bool{}
	for _, a := range p.Platform.Arch {
		if a != "x64" && a != "x86" && a != "arm64" || archSeen[a] {
			return fmt.Errorf("E-CAT-003 %s: arch inválida ou duplicada", p.ID)
		}
		archSeen[a] = true
	}
	if err := safeRelative(p.Payload.Path); err != nil {
		return err
	}
	if !strings.HasPrefix(p.Payload.Path, "payloads/"+p.ID+"/") && p.Payload.Path != "payloads/"+p.ID {
		return fmt.Errorf("E-CAT-005 %s: payload fora da pasta de id", p.ID)
	}
	switch p.Payload.Kind {
	case "file", "archive":
		if p.Payload.Size <= 0 || !hashPattern.MatchString(p.Payload.SHA256) || p.Payload.Entry != "" || len(p.Payload.Files) > 0 {
			return fmt.Errorf("E-CAT-003 %s: tamanho/hash de payload inválido", p.ID)
		}
	case "dir":
		if p.Payload.Entry == "" || len(p.Payload.Files) == 0 || p.Payload.SHA256 != "" || p.Payload.Size != 0 {
			return fmt.Errorf("E-CAT-003 %s: diretório sem entry e files", p.ID)
		}
		if err := safeRelative(p.Payload.Entry); err != nil {
			return err
		}
		seen := map[string]bool{}
		foundEntry := false
		for _, f := range p.Payload.Files {
			if err := safeRelative(f.Path); err != nil {
				return err
			}
			if seen[f.Path] || f.Size <= 0 || !hashPattern.MatchString(f.SHA256) {
				return fmt.Errorf("E-CAT-003 %s: arquivo inválido ou duplicado", p.ID)
			}
			seen[f.Path] = true
			if f.Path == p.Payload.Entry {
				foundEntry = true
			}
		}
		if !foundEntry {
			return fmt.Errorf("E-CAT-003 %s: entry não está na lista de files", p.ID)
		}
	default:
		return fmt.Errorf("E-CAT-003 %s: payload.kind desconhecido", p.ID)
	}
	switch p.Install.Driver {
	case "msi", "exe", "script":
		if p.Payload.Kind == "archive" {
			return fmt.Errorf("E-CAT-003 %s: archive somente portable", p.ID)
		}
		if p.Install.Target != "" || p.Install.AddToPath || len(p.Install.Shortcuts) > 0 || p.Install.UninstallEntry != nil {
			return fmt.Errorf("E-CAT-003 %s: campos portable em outro driver", p.ID)
		}
		if p.Install.Driver == "script" && p.Payload.Kind == "file" && p.Payload.Entry != "" {
			return fmt.Errorf("E-CAT-003 %s: entry não permitido em file", p.ID)
		}
	case "portable":
		if p.Payload.Kind != "archive" || p.Install.Target == "" || p.Install.RawCmdLine != "" || len(p.Install.Args) > 0 {
			return fmt.Errorf("E-CAT-003 %s: install portable inválido", p.ID)
		}
		for _, s := range p.Install.Shortcuts {
			if s.Name == "" || (s.Location != "" && s.Location != "desktop" && s.Location != "startMenu") {
				return fmt.Errorf("E-CAT-003: shortcut inválido")
			}
			if err := safeRelative(s.Target); err != nil {
				return err
			}
		}
	case "msix":
		return fmt.Errorf("E-CAT-008 %s: driver msix previsto para fase 4, ainda indisponível", p.ID)
	default:
		return fmt.Errorf("E-CAT-008 %s: driver desconhecido", p.ID)
	}
	if p.Install.TimeoutSec <= 0 || len(p.Install.SuccessCodes) == 0 {
		return fmt.Errorf("E-CAT-002 %s: timeoutSec/successCodes obrigatório", p.ID)
	}
	if p.Install.RawCmdLine != "" && len(p.Install.Args) > 0 {
		return fmt.Errorf("E-CAT-003 %s: args e rawCmdLine são exclusivos", p.ID)
	}
	if err := validateArgs(p.Install.Args); err != nil {
		return err
	}
	// A Windows exit code must have exactly one interpretation.
	// Overlapping success/reboot/retry lists would make retry behavior ambiguous.
	classes := map[int]string{}
	for _, group := range []struct {
		name string
		codes []int
	}{
		{name: "successCodes", codes: p.Install.SuccessCodes},
		{name: "rebootCodes", codes: p.Install.RebootCodes},
		{name: "retryCodes", codes: p.Install.RetryCodes},
	} {
		for _, code := range group.codes {
			if previous, ok := classes[code]; ok {
				return fmt.Errorf("E-CAT-003 %s: exit code %d presente em %s e %s", p.ID, code, previous, group.name)
			}
			classes[code] = group.name
		}
	}
	if p.Detect.Match != "any" && p.Detect.Match != "all" || len(p.Detect.Rules) == 0 {
		return fmt.Errorf("E-CAT-002 %s: detect inválido", p.ID)
	}
	for _, r := range p.Detect.Rules {
		if r.MinVersion != "" && !domain.ValidVersion(r.MinVersion) {
			return fmt.Errorf("E-CAT-003: minVersion inválida")
		}
		switch r.Type {
		case "registry-uninstall":
			if r.DisplayName == "" {
				return fmt.Errorf("E-CAT-002: displayName obrigatório")
			}
		case "msi-product-code":
			if !guidPattern.MatchString(r.ProductCode) {
				return fmt.Errorf("E-CAT-003: productCode inválido")
			}
		case "file":
			if r.Path == "" {
				return fmt.Errorf("E-CAT-002: file.path obrigatório")
			}
		case "registry-value":
			if !strings.HasPrefix(r.Key, "HKLM\\") && !strings.HasPrefix(r.Key, "HKCU\\") || r.Value == "" {
				return fmt.Errorf("E-CAT-003: registry-value inválido")
			}
			n := 0
			if len(r.Equals) > 0 {
				n++
			}
			if r.AtLeast != nil {
				n++
			}
			if r.MinVersion != "" {
				n++
			}
			if n > 1 {
				return fmt.Errorf("E-CAT-003: comparadores registry-value exclusivos")
			}
		case "command":
			if r.Command == "" || r.Matches == "" || r.TimeoutSec < 0 || r.TimeoutSec > 120 {
				return fmt.Errorf("E-CAT-003: detect.command inválido")
			}
			if _, err := regexp.Compile(r.Matches); err != nil {
				return fmt.Errorf("E-CAT-003: regex inválida: %w", err)
			}
		case "appx":
			if r.Name == "" {
				return fmt.Errorf("E-CAT-002: appx.name obrigatório")
			}
		default:
			return fmt.Errorf("E-CAT-009: tipo de detecção desconhecido %q", r.Type)
		}
	}
	if p.Uninstall != nil {
		switch p.Uninstall.Driver {
		case "msi", "exe", "portable", "script":
		default:
			return fmt.Errorf("E-CAT-008: uninstall.driver desconhecido")
		}
		if (p.Uninstall.Driver == "exe" || p.Uninstall.Driver == "script") && p.Uninstall.Command == "" {
			return fmt.Errorf("E-CAT-002: uninstall.command obrigatório")
		}
		if err := validateArgs(p.Uninstall.Args); err != nil {
			return err
		}
	}
	return nil
}
func validateArgs(args []json.RawMessage) error {
	for _, raw := range args {
		var s string
		if json.Unmarshal(raw, &s) == nil {
			if strings.Contains(s, "{secret}") {
				return fmt.Errorf("E-CAT-011: marcador secret sem objeto")
			}
			continue
		}
		var v struct {
			Value  string `json:"value"`
			Secret string `json:"secret"`
		}
		if err := json.Unmarshal(raw, &v); err != nil || v.Secret == "" || !validID(v.Secret) || !strings.Contains(v.Value, "{secret}") {
			return fmt.Errorf("E-CAT-011: argumento secret inválido")
		}
		var fields map[string]any
		_ = json.Unmarshal(raw, &fields)
		if len(fields) != 2 {
			return fmt.Errorf("E-CAT-003: argumento secret com campo desconhecido")
		}
	}
	return nil
}
func validateReferences(c domain.Catalog) error {
	for id, p := range c.Packages {
		for _, x := range append(append([]string{}, p.DependsOn...), p.ConflictsWith...) {
			if _, ok := c.Packages[x]; !ok {
				return fmt.Errorf("E-CAT-006 %s: referência de pacote inexistente %s", id, x)
			}
			if x == id {
				return fmt.Errorf("E-CAT-007: pacote %s depende ou conflita consigo mesmo", id)
			}
		}
	}
	for id, p := range c.Profiles {
		if p.Extends != "" {
			if _, ok := c.Profiles[p.Extends]; !ok {
				return fmt.Errorf("E-CAT-006 %s: extends inexistente %s", id, p.Extends)
			}
		}
		seen := map[string]bool{}
		for _, x := range p.Packages {
			if _, ok := c.Packages[x]; !ok {
				return fmt.Errorf("E-CAT-006 %s: pacote inexistente %s", id, x)
			}
			if seen[x] {
				return fmt.Errorf("E-CAT-003 %s: pacote repetido %s", id, x)
			}
			seen[x] = true
		}
	}
	states := map[string]int{}
	var visitPackage func(string) error
	visitPackage = func(id string) error {
		if states[id] == 1 {
			return fmt.Errorf("E-CAT-007: ciclo de dependência em %s", id)
		}
		if states[id] == 2 {
			return nil
		}
		states[id] = 1
		for _, dep := range c.Packages[id].DependsOn {
			if err := visitPackage(dep); err != nil {
				return err
			}
		}
		states[id] = 2
		return nil
	}
	for id := range c.Packages {
		if err := visitPackage(id); err != nil {
			return err
		}
	}
	states = map[string]int{}
	var visitProfile func(string) error
	visitProfile = func(id string) error {
		if states[id] == 1 {
			return fmt.Errorf("E-CAT-007: ciclo de extends em %s", id)
		}
		if states[id] == 2 {
			return nil
		}
		states[id] = 1
		if p := c.Profiles[id]; p.Extends != "" {
			if err := visitProfile(p.Extends); err != nil {
				return err
			}
		}
		states[id] = 2
		return nil
	}
	for id := range c.Profiles {
		if err := visitProfile(id); err != nil {
			return err
		}
	}
	return nil
}
