package daemon

import (
	"encoding/json"
	"path/filepath"

	"github.com/elionaz/panal/internal/client"
	"github.com/elionaz/panal/internal/hooks"
	"github.com/elionaz/panal/internal/settings"
)

// MCP server names agents see.
const (
	mcpPanal   = "panal"
	mcpBrowser = "panal-browser"
)

// mcpOn reports whether agent sessions get panal's MCP server (settings
// mcp.enabled).
func (s *Server) mcpOn() bool {
	st, _ := settings.Load(s.cfg.Paths.Settings())
	return st.MCP.Enabled
}

// cliExe runs the MCP servers' CLI commands: the daemon's own
// executable, or, when that is panal.app's (an app install), the CLI
// bundled next to it, since the app's executable only runs the daemon and
// hooks (see client.CLIExecutable).
func (s *Server) cliExe() string {
	return client.CLIExecutable(s.cfg.Exe)
}

// panalMCP is `panal mcp` bound to this daemon.
func (s *Server) panalMCP() hooks.MCPServer {
	return hooks.MCPServer{Command: s.cliExe(), Args: []string{"mcp", "--sock", s.sock}}
}

// hookMCP are the MCP servers an agent's hook integration carries (see
// hooks.WithMCP): panal's own, for the agents that take it that way.
func (s *Server) hookMCP(agent string) map[string]hooks.MCPServer {
	switch agent {
	case hooks.AgentGemini, hooks.AgentOpencode, hooks.AgentCursor:
		if s.mcpOn() {
			return map[string]hooks.MCPServer{mcpPanal: s.panalMCP()}
		}
	}
	return nil
}

// agentMCPArgs are the arguments that give a Claude Code or codex
// session its MCP servers, for that invocation only: panal's own (unless
// mcp.enabled is off) and the browser one (`panal browser mcp`, when the
// browser is on and has Chrome and npx). nil when there are none.
func (s *Server) agentMCPArgs(agent string) []string {
	if agent != hooks.AgentClaude && agent != hooks.AgentCodex {
		return nil
	}
	servers := map[string]hooks.MCPServer{}
	if s.mcpOn() {
		servers[mcpPanal] = s.panalMCP()
	}
	if s.browserReady() {
		servers[mcpBrowser] = hooks.MCPServer{Command: s.cliExe(), Args: []string{"browser", "mcp", "--sock", s.sock}}
	}
	if len(servers) == 0 {
		return nil
	}
	if agent == hooks.AgentClaude {
		list := map[string]any{}
		for name, m := range servers {
			list[name] = map[string]any{"type": "stdio", "command": m.Command, "args": m.Args}
		}
		cfg, _ := json.Marshal(map[string]any{"mcpServers": list})
		// Claude reads it at start: rewriting it later (the browser turned
		// on or off) does not touch running sessions.
		file := filepath.Join(s.cfg.Paths.HooksDir(), "mcp.json")
		if err := hooks.WriteIfChanged(file, append(cfg, '\n')); err != nil {
			s.log.Warn("MCP config", "err", err)
			return nil
		}
		// Before --settings: --mcp-config takes several values, a flag
		// ends them (a profile's positional prompt must not become one).
		return []string{"--mcp-config", file}
	}
	var args []string
	for _, name := range []string{mcpPanal, mcpBrowser} {
		m, ok := servers[name]
		if !ok {
			continue
		}
		cmd, _ := json.Marshal(m.Command)
		a, _ := json.Marshal(m.Args)
		args = append(args,
			"-c", "mcp_servers."+name+".command="+string(cmd),
			"-c", "mcp_servers."+name+".args="+string(a))
	}
	return args
}
