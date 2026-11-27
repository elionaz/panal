package daemon_test

import (
	"os"
	"strings"
	"testing"

	"github.com/elionaz/panal/internal/protocol"
)

// A spawn that fails is in the daemon log (with what was asked for), not
// only in the error its caller got.
func TestFailedSpawnIsLogged(t *testing.T) {
	c, _, logs, _ := startLoggedDaemon(t)
	ctx := ctxT(t)
	dir, err := os.MkdirTemp("/tmp", "agt-nogit")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	proj, err := c.ProjectAdd(ctx, dir, "nogit")
	if err != nil {
		t.Fatal(err)
	}
	// A worktree in a folder without git.
	if _, err := c.Spawn(ctx, protocol.SpawnParams{ProjectID: proj.ID, ProfileID: "shell", Mode: protocol.ModeWorktree}); err == nil {
		t.Fatal("worktree spawn without git succeeded")
	}
	// The log line is written before the reply is sent.
	var line string
	for _, l := range strings.Split(logs.String(), "\n") {
		if strings.Contains(l, "session spawn failed") {
			line = l
		}
	}
	for _, want := range []string{"project=" + proj.ID, "profile=shell", "mode=worktree", "err="} {
		if !strings.Contains(line, want) {
			t.Errorf("log line %q lacks %q", line, want)
		}
	}
	// A spawn that works logs no failure.
	before := strings.Count(logs.String(), "session spawn failed")
	if _, err := c.Spawn(ctx, protocol.SpawnParams{ProjectID: proj.ID, ProfileID: "shell"}); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(logs.String(), "session spawn failed"); n != before {
		t.Errorf("a successful spawn logged a failure (%d, was %d)", n, before)
	}
}
