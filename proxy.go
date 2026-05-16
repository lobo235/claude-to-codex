package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type childServer struct {
	ScopedServer
	session *mcpsdk.ClientSession
	cancel  context.CancelFunc
}

type childConnection struct {
	session *mcpsdk.ClientSession
	cancel  context.CancelFunc
}

type proxyServer struct {
	children         []*childServer
	tools            map[string]toolRoute
	prompts          map[string]promptRoute
	resource         map[string]*childServer
	templateRoutes   []*childServer
	logger           *slog.Logger
	operationTimeout time.Duration
}

type childFailure struct {
	server    ScopedServer
	operation string
	err       error
}

type toolRoute struct {
	child *childServer
	name  string
}

type promptRoute struct {
	child *childServer
	name  string
}

type exposedTool struct {
	exposedName  string
	originalName string
	server       ScopedServer
}

func newProxyServer(logger *slog.Logger) *proxyServer {
	return &proxyServer{
		tools:            map[string]toolRoute{},
		prompts:          map[string]promptRoute{},
		resource:         map[string]*childServer{},
		logger:           logger,
		operationTimeout: bridgeOperationTimeout(),
	}
}

func (p *proxyServer) connectChildren(ctx context.Context, servers []ScopedServer) error {
	failures := p.connectChildrenBestEffort(ctx, servers)
	if len(failures) == 0 {
		return nil
	}
	if len(p.children) > 0 {
		for _, failure := range failures {
			p.logger.Warn("skipping unavailable Claude MCP server", "scope", failure.server.Scope, "name", failure.server.Name, "operation", failure.operation, "error", redactSensitive(failure.err.Error()))
		}
		return nil
	}
	return connectFailuresError(failures)
}

func (p *proxyServer) connectChildrenBestEffort(ctx context.Context, servers []ScopedServer) []childFailure {
	var failures []childFailure
	for _, server := range servers {
		childCtx, cancel := p.operationContext(ctx)
		conn, err := connectChild(childCtx, server)
		cancel()
		if err != nil {
			failures = append(failures, childFailure{server: server, operation: "connect", err: err})
			continue
		}
		p.children = append(p.children, &childServer{ScopedServer: server, session: conn.session, cancel: conn.cancel})
		p.logger.Info("connected Claude MCP server", "scope", server.Scope, "name", server.Name)
	}
	return failures
}

func (p *proxyServer) operationContext(parent context.Context) (context.Context, context.CancelFunc) {
	timeout := p.operationTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return context.WithTimeout(parent, timeout)
}

func connectChild(ctx context.Context, server ScopedServer) (*childConnection, error) {
	sessionCtx, cancel := context.WithCancel(context.Background())
	result := make(chan struct {
		session *mcpsdk.ClientSession
		err     error
	}, 1)
	go func() {
		session, err := connectChildSession(sessionCtx, server)
		result <- struct {
			session *mcpsdk.ClientSession
			err     error
		}{session: session, err: err}
	}()
	select {
	case res := <-result:
		if res.err != nil {
			cancel()
			return nil, res.err
		}
		return &childConnection{session: res.session, cancel: cancel}, nil
	case <-ctx.Done():
		cancel()
		return nil, ctx.Err()
	}
}

func connectChildSession(ctx context.Context, server ScopedServer) (*mcpsdk.ClientSession, error) {
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "claude-to-codex", Version: version}, nil)
	cfg := server.Config
	if cfg.URL != "" || strings.EqualFold(cfg.Type, "http") || strings.EqualFold(cfg.Type, "streamable-http") {
		httpClient := http.DefaultClient
		if len(cfg.Headers) > 0 {
			httpClient = &http.Client{Transport: headerTransport{headers: cfg.Headers, base: http.DefaultTransport}}
		}
		return client.Connect(ctx, &mcpsdk.StreamableClientTransport{Endpoint: cfg.URL, HTTPClient: httpClient, DisableStandaloneSSE: true}, nil)
	}
	if cfg.Command == "" {
		return nil, fmt.Errorf("missing command or url")
	}
	cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)
	cmd.Env = buildChildEnv(server)
	if server.WorkDir != "" {
		cmd.Dir = server.WorkDir
	}
	return client.Connect(ctx, &mcpsdk.CommandTransport{Command: cmd, TerminateDuration: 2 * time.Second}, nil)
}

func connectFailuresError(failures []childFailure) error {
	var parts []string
	for _, failure := range failures {
		parts = append(parts, fmt.Sprintf("%s-scope MCP server %q %s: %s", failure.server.Scope, failure.server.Name, failure.operation, redactSensitive(failure.err.Error())))
	}
	return fmt.Errorf("no Claude MCP child servers available: %s", strings.Join(parts, "; "))
}

type headerTransport struct {
	headers map[string]string
	base    http.RoundTripper
}

func (t headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	for key, value := range t.headers {
		clone.Header.Set(key, value)
	}
	return t.base.RoundTrip(clone)
}

func (p *proxyServer) serverOptions() *mcpsdk.ServerOptions {
	return &mcpsdk.ServerOptions{
		CompletionHandler: func(ctx context.Context, req *mcpsdk.CompleteRequest) (*mcpsdk.CompleteResult, error) {
			if req.Params.Ref == nil {
				return nil, fmt.Errorf("completion reference is required")
			}
			params := *req.Params
			switch params.Ref.Type {
			case "ref/prompt":
				route, ok := p.prompts[params.Ref.Name]
				if !ok {
					return nil, fmt.Errorf("unknown prompt completion reference %q", params.Ref.Name)
				}
				ref := *params.Ref
				ref.Name = route.name
				params.Ref = &ref
				return route.child.session.Complete(ctx, &params)
			case "ref/resource":
				child := p.resource[params.Ref.URI]
				if child == nil {
					return nil, fmt.Errorf("unknown resource completion reference %q", params.Ref.URI)
				}
				return child.session.Complete(ctx, &params)
			default:
				return nil, fmt.Errorf("unsupported completion reference type %q", params.Ref.Type)
			}
		},
		SubscribeHandler: func(ctx context.Context, req *mcpsdk.SubscribeRequest) error {
			child := p.resource[req.Params.URI]
			if child == nil {
				return fmt.Errorf("unknown resource subscription %q", req.Params.URI)
			}
			return child.session.Subscribe(ctx, req.Params)
		},
		UnsubscribeHandler: func(ctx context.Context, req *mcpsdk.UnsubscribeRequest) error {
			child := p.resource[req.Params.URI]
			if child == nil {
				return fmt.Errorf("unknown resource subscription %q", req.Params.URI)
			}
			return child.session.Unsubscribe(ctx, req.Params)
		},
	}
}

func (p *proxyServer) register(ctx context.Context, srv *mcpsdk.Server) error {
	var toolNames []string
	var promptNames []string

	childTools := map[*childServer][]*mcpsdk.Tool{}
	childPrompts := map[*childServer][]*mcpsdk.Prompt{}

	for _, child := range p.children {
		toolsCtx, cancelTools := p.operationContext(ctx)
		tools, err := collectTools(toolsCtx, child.session)
		cancelTools()
		if err != nil {
			p.logger.Warn("child tools unavailable", "scope", child.Scope, "server", child.Name, "error", redactSensitive(err.Error()))
		} else {
			childTools[child] = tools
			for _, tool := range tools {
				toolNames = append(toolNames, tool.Name)
			}
		}

		promptsCtx, cancelPrompts := p.operationContext(ctx)
		prompts, err := collectPrompts(promptsCtx, child.session)
		cancelPrompts()
		if err == nil {
			childPrompts[child] = prompts
			for _, prompt := range prompts {
				promptNames = append(promptNames, prompt.Name)
			}
		} else {
			p.logger.Debug("child prompts unavailable", "server", child.Name, "error", redactSensitive(err.Error()))
		}
	}

	toolMapper := newNameMapper(toolNames)
	for child, tools := range childTools {
		for _, childTool := range tools {
			exposed := toolMapper.exposed(child.Name, childTool.Name)
			tool := *childTool
			tool.Name = exposed
			if tool.InputSchema == nil {
				tool.InputSchema = map[string]any{"type": "object"}
			}
			original := childTool.Name
			route := toolRoute{child: child, name: original}
			p.tools[exposed] = route
			srv.AddTool(&tool, func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
				return route.child.session.CallTool(ctx, &mcpsdk.CallToolParams{Name: route.name, Arguments: req.Params.Arguments})
			})
		}
	}

	promptMapper := newNameMapper(promptNames)
	for child, prompts := range childPrompts {
		for _, childPrompt := range prompts {
			exposed := promptMapper.exposed(child.Name, childPrompt.Name)
			prompt := *childPrompt
			prompt.Name = exposed
			original := childPrompt.Name
			route := promptRoute{child: child, name: original}
			p.prompts[exposed] = route
			srv.AddPrompt(&prompt, func(ctx context.Context, req *mcpsdk.GetPromptRequest) (*mcpsdk.GetPromptResult, error) {
				params := *req.Params
				params.Name = route.name
				return route.child.session.GetPrompt(ctx, &params)
			})
		}
	}

	for _, child := range p.children {
		resourcesCtx, cancelResources := p.operationContext(ctx)
		resources, err := collectResources(resourcesCtx, child.session)
		cancelResources()
		if err == nil {
			for _, childResource := range resources {
				resource := *childResource
				resourceChild := child
				p.resource[resource.URI] = resourceChild
				srv.AddResource(&resource, func(ctx context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
					return resourceChild.session.ReadResource(ctx, req.Params)
				})
			}
		} else {
			p.logger.Debug("child resources unavailable", "server", child.Name, "error", redactSensitive(err.Error()))
		}

		templatesCtx, cancelTemplates := p.operationContext(ctx)
		templates, err := collectResourceTemplates(templatesCtx, child.session)
		cancelTemplates()
		if err == nil {
			for _, childTemplate := range templates {
				template := *childTemplate
				templateChild := child
				p.templateRoutes = append(p.templateRoutes, templateChild)
				srv.AddResourceTemplate(&template, func(ctx context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
					res, err := templateChild.session.ReadResource(ctx, req.Params)
					if err != nil {
						return nil, err
					}
					return res, nil
				})
			}
		} else {
			p.logger.Debug("child resource templates unavailable", "server", child.Name, "error", redactSensitive(err.Error()))
		}
	}

	return nil
}

func (p *proxyServer) inspectTools(ctx context.Context) ([]exposedTool, []childFailure) {
	childTools := map[*childServer][]*mcpsdk.Tool{}
	var toolNames []string
	var failures []childFailure
	for _, child := range p.children {
		toolsCtx, cancel := p.operationContext(ctx)
		tools, err := collectTools(toolsCtx, child.session)
		cancel()
		if err != nil {
			failures = append(failures, childFailure{server: child.ScopedServer, operation: "list_tools", err: err})
			continue
		}
		childTools[child] = tools
		for _, tool := range tools {
			toolNames = append(toolNames, tool.Name)
		}
	}
	mapper := newNameMapper(toolNames)
	var out []exposedTool
	for child, tools := range childTools {
		for _, tool := range tools {
			out = append(out, exposedTool{
				exposedName:  mapper.exposed(child.Name, tool.Name),
				originalName: tool.Name,
				server:       child.ScopedServer,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].exposedName < out[j].exposedName
	})
	return out, failures
}

func collectTools(ctx context.Context, session *mcpsdk.ClientSession) ([]*mcpsdk.Tool, error) {
	var tools []*mcpsdk.Tool
	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			return nil, err
		}
		tools = append(tools, tool)
	}
	return tools, nil
}

func collectPrompts(ctx context.Context, session *mcpsdk.ClientSession) ([]*mcpsdk.Prompt, error) {
	var prompts []*mcpsdk.Prompt
	for prompt, err := range session.Prompts(ctx, nil) {
		if err != nil {
			return nil, err
		}
		prompts = append(prompts, prompt)
	}
	return prompts, nil
}

func collectResources(ctx context.Context, session *mcpsdk.ClientSession) ([]*mcpsdk.Resource, error) {
	var resources []*mcpsdk.Resource
	for resource, err := range session.Resources(ctx, nil) {
		if err != nil {
			return nil, err
		}
		resources = append(resources, resource)
	}
	return resources, nil
}

func collectResourceTemplates(ctx context.Context, session *mcpsdk.ClientSession) ([]*mcpsdk.ResourceTemplate, error) {
	var templates []*mcpsdk.ResourceTemplate
	for template, err := range session.ResourceTemplates(ctx, nil) {
		if err != nil {
			return nil, err
		}
		templates = append(templates, template)
	}
	return templates, nil
}

func (p *proxyServer) close() {
	var wg sync.WaitGroup
	for _, child := range p.children {
		wg.Add(1)
		go func(child *childServer) {
			defer wg.Done()
			_ = child.session.Close()
			if child.cancel != nil {
				child.cancel()
			}
		}(child)
	}
	wg.Wait()
}
