package daemon

import (
	"fmt"

	"github.com/elionaz/panal/internal/profile"
	"github.com/elionaz/panal/internal/protocol"
)

// profileInstall runs catalog agent p.ProfileID's install command in a new
// shell session of project p.ProjectID: in its terminal, in the login
// shell, so its output (and a sudo or keychain prompt) are in view; the
// shell stays open afterwards. Only commands from the built-in catalog are
// run, never a profiles.yaml value. A successful install rescans the
// agents (panal profiles scan, which tells the UIs).
func (s *Server) profileInstall(p protocol.ProfileInstallParams) (protocol.SessionInfo, error) {
	name, cmd := catalogInstall(p.ProfileID)
	if cmd == "" {
		return protocol.SessionInfo{}, fmt.Errorf("%s is not a catalog agent panal knows how to install", p.ProfileID)
	}
	if ps, err := s.profiles.List(); err == nil {
		if cur, ok := profile.Find(ps, p.ProfileID); ok && cur.Installed {
			return protocol.SessionInfo{}, fmt.Errorf("%s is already installed (%s)", name, cur.Path)
		}
	}
	// The rescan never decides the outcome: the install's status does.
	// It is a CLI command: the bundled CLI in an app install (cliExe).
	rescan := "{ " + shellQuote(s.cliExe()) + " profiles scan >/dev/null 2>&1 || true; }"
	info, err := s.spawnWith(protocol.SpawnParams{ProjectID: p.ProjectID, ProfileID: "shell", Cols: p.Cols, Rows: p.Rows},
		spawnExtra{setup: "( " + cmd + " ) && " + rescan, setupTitle: "installing " + name})
	if err != nil {
		return info, err
	}
	s.setName(info.ID, "install "+p.ProfileID)
	info.Name = "install " + p.ProfileID
	s.log.Info("agent install started", "profile", p.ProfileID, "session", info.ID, "command", cmd)
	return info, nil
}

// catalogInstall is profile.CatalogInstall (tests swap it: they must
// never run a real install).
var catalogInstall = profile.CatalogInstall

// shellQuote single-quotes s for sh.
func shellQuote(s string) string {
	out := "'"
	for _, r := range s {
		if r == '\'' {
			out += `'\''`
		} else {
			out += string(r)
		}
	}
	return out + "'"
}
