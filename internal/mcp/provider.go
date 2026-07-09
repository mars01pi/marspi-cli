package mcp

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mars/marspi-cli/internal/tool"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const callTimeout = 60 * time.Second

var nonNameChar = regexp.MustCompile(`[^a-zA-Z0-9_.-]`)

type serverSession struct {
	session *sdkmcp.ClientSession
}

// Provider 是 MCP 工具提供器（当前支持 stdio tools/list + tools/call）。
type Provider struct {
	cfg      Config
	mu       sync.RWMutex
	tools    []tool.Tool
	sessions map[string]*serverSession
}

func NewProvider(cfg Config) *Provider {
	return &Provider{cfg: cfg, sessions: map[string]*serverSession{}}
}

func (p *Provider) ID() string { return "mcp" }

func (p *Provider) Tools() []tool.Tool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return append([]tool.Tool(nil), p.tools...)
}

func (p *Provider) Refresh(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	nextSessions := make(map[string]*serverSession, len(p.cfg.MCPServers))
	nextTools := make([]tool.Tool, 0, len(p.cfg.MCPServers)*8)

	for serverName, serverCfg := range p.cfg.MCPServers {
		if strings.TrimSpace(serverCfg.Command) == "" {
			continue
		}
		ss := p.sessions[serverName]
		if ss == nil {
			connected, err := connectServer(ctx, serverCfg)
			if err != nil {
				continue
			}
			ss = connected
		}
		tools, err := listAllTools(ctx, ss.session)
		if err != nil {
			_ = ss.session.Close()
			continue
		}
		nextSessions[serverName] = ss
		for _, mt := range tools {
			nextTools = append(nextTools, newToolAdapter(serverName, ss.session, mt))
		}
	}

	for name, old := range p.sessions {
		if _, ok := nextSessions[name]; ok {
			continue
		}
		_ = old.session.Close()
	}

	p.sessions = nextSessions
	p.tools = nextTools
	return nil
}

func (p *Provider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	var firstErr error
	for _, ss := range p.sessions {
		if err := ss.session.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	p.sessions = map[string]*serverSession{}
	p.tools = nil
	return firstErr
}

type mcpTool struct {
	tool.Base
	server  string
	rawName string
	full    string
	desc    string
	params  []tool.Param
	session *sdkmcp.ClientSession
}

func newToolAdapter(serverName string, session *sdkmcp.ClientSession, mt *sdkmcp.Tool) *mcpTool {
	return &mcpTool{
		server:  serverName,
		rawName: mt.Name,
		full:    qualifiedName(serverName, mt.Name),
		desc:    strings.TrimSpace(mt.Description),
		params:  paramsFromSchema(mt.InputSchema),
		session: session,
	}
}

func (t *mcpTool) Name() string { return t.full }
func (t *mcpTool) Description() string {
	return ifEmpty(t.desc, "MCP tool from server "+t.server)
}
func (t *mcpTool) Params() []tool.Param {
	return append([]tool.Param(nil), t.params...)
}
func (t *mcpTool) Preview(args map[string]any) string {
	if len(args) == 0 {
		return "[" + t.server + "] " + t.rawName
	}
	b, _ := json.Marshal(args)
	return "[" + t.server + "] " + t.rawName + " " + truncateString(string(b), 120)
}
func (t *mcpTool) Run(args map[string]any) tool.Result {
	ctx, cancel := context.WithTimeout(context.Background(), callTimeout)
	defer cancel()
	res, err := t.session.CallTool(ctx, &sdkmcp.CallToolParams{Name: t.rawName, Arguments: args})
	if err != nil {
		return tool.Fail("mcp call failed: " + err.Error())
	}
	out := renderCallResult(res)
	if res.IsError {
		return tool.Fail(out)
	}
	return tool.OK(out)
}

func connectServer(ctx context.Context, cfg ServerConfig) (*serverSession, error) {
	client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "marspi-cli", Version: "0.1.0"}, nil)
	cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)
	cmd.Env = append([]string{}, os.Environ()...)
	for k, v := range cfg.Env {
		cmd.Env = append(cmd.Env, k+"="+expandEnv(v))
	}
	session, err := client.Connect(ctx, &sdkmcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		return nil, err
	}
	return &serverSession{session: session}, nil
}

func listAllTools(ctx context.Context, session *sdkmcp.ClientSession) ([]*sdkmcp.Tool, error) {
	var (
		cursor string
		all    []*sdkmcp.Tool
	)
	for {
		res, err := session.ListTools(ctx, &sdkmcp.ListToolsParams{Cursor: cursor})
		if err != nil {
			return nil, err
		}
		all = append(all, res.Tools...)
		if strings.TrimSpace(res.NextCursor) == "" {
			return all, nil
		}
		cursor = res.NextCursor
	}
}

func qualifiedName(serverName, rawToolName string) string {
	return "mcp__" + sanitizeName(serverName) + "__" + sanitizeName(rawToolName)
}

func sanitizeName(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "tool"
	}
	return nonNameChar.ReplaceAllString(s, "_")
}

func paramsFromSchema(input any) []tool.Param {
	schema, ok := input.(map[string]any)
	if !ok {
		return nil
	}
	properties, _ := schema["properties"].(map[string]any)
	if len(properties) == 0 {
		return nil
	}
	required := map[string]bool{}
	if req, ok := schema["required"].([]any); ok {
		for _, v := range req {
			if s, ok := v.(string); ok {
				required[s] = true
			}
		}
	}
	out := make([]tool.Param, 0, len(properties))
	for name, p := range properties {
		param := tool.Param{Name: name, Type: "string"}
		if m, ok := p.(map[string]any); ok {
			if s, ok := m["description"].(string); ok {
				param.Desc = s
			}
			if typ, ok := m["type"].(string); ok {
				switch typ {
				case "integer", "number":
					param.Type = "number"
				case "boolean":
					param.Type = "boolean"
				default:
					param.Type = "string"
				}
			}
		}
		if !required[name] {
			param.Type += "?"
		}
		out = append(out, param)
	}
	return out
}

func renderCallResult(res *sdkmcp.CallToolResult) string {
	if res == nil {
		return ""
	}
	lines := make([]string, 0, len(res.Content)+1)
	for _, c := range res.Content {
		switch v := c.(type) {
		case *sdkmcp.TextContent:
			if strings.TrimSpace(v.Text) != "" {
				lines = append(lines, v.Text)
			}
		case *sdkmcp.ImageContent:
			lines = append(lines, "[image:"+v.MIMEType+" bytes="+strconv.Itoa(len(v.Data))+"]")
		case *sdkmcp.AudioContent:
			lines = append(lines, "[audio:"+v.MIMEType+" bytes="+strconv.Itoa(len(v.Data))+"]")
		default:
			b, _ := json.Marshal(v)
			if len(b) > 0 {
				lines = append(lines, string(b))
			}
		}
	}
	if len(lines) > 0 {
		return strings.Join(lines, "\n")
	}
	if res.StructuredContent != nil {
		if b, err := json.Marshal(res.StructuredContent); err == nil {
			return string(b)
		}
	}
	return ""
}

func expandEnv(s string) string {
	return os.Expand(s, func(key string) string { return os.Getenv(key) })
}

func truncateString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func ifEmpty(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}
