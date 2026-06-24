package bridge

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
	mu          sync.RWMutex
	reconnectMu sync.Mutex
	session     *mcpsdk.ClientSession
	cancel      context.CancelFunc
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
	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "claude-to-codex", Version: Version}, nil)
	cfg, err := expandMCPServerConfig(server.Config)
	if err != nil {
		return nil, fmt.Errorf("expand config: %w", err)
	}
	server.Config = cfg
	if strings.EqualFold(cfg.Type, "sse") {
		if cfg.URL == "" {
			return nil, fmt.Errorf("missing url")
		}
		httpClient := httpClientWithHeaders(cfg.Headers)
		return client.Connect(ctx, &mcpsdk.SSEClientTransport{Endpoint: cfg.URL, HTTPClient: httpClient}, nil)
	}
	if cfg.URL != "" || strings.EqualFold(cfg.Type, "http") || strings.EqualFold(cfg.Type, "streamable-http") {
		httpClient := httpClientWithHeaders(cfg.Headers)
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

func httpClientWithHeaders(headers map[string]string) *http.Client {
	if len(headers) == 0 {
		return http.DefaultClient
	}
	return &http.Client{Transport: headerTransport{headers: headers, base: http.DefaultTransport}}
}

func connectFailuresError(failures []childFailure) error {
	var parts []string
	for _, failure := range failures {
		parts = append(parts, fmt.Sprintf("%s-scope MCP server %q %s: %s", failure.server.Scope, failure.server.Name, failure.operation, redactSensitive(failure.err.Error())))
	}
	return fmt.Errorf("no Claude MCP child servers available: %s", strings.Join(parts, "; "))
}

func (child *childServer) currentSession() *mcpsdk.ClientSession {
	child.mu.RLock()
	defer child.mu.RUnlock()
	return child.session
}

func (child *childServer) replaceConnection(conn *childConnection) (*mcpsdk.ClientSession, context.CancelFunc) {
	child.mu.Lock()
	defer child.mu.Unlock()
	oldSession := child.session
	oldCancel := child.cancel
	child.session = conn.session
	child.cancel = conn.cancel
	return oldSession, oldCancel
}

func (child *childServer) close() {
	session, cancel := child.replaceConnection(&childConnection{})
	if session != nil {
		_ = session.Close()
	}
	if cancel != nil {
		cancel()
	}
}

func (p *proxyServer) reconnectChild(ctx context.Context, child *childServer, operation string) error {
	child.reconnectMu.Lock()
	defer child.reconnectMu.Unlock()

	p.logger.Warn("reconnecting Claude MCP server after child connection failure", "scope", child.Scope, "server", child.Name, "operation", operation)
	reconnectCtx, cancel := p.operationContext(ctx)
	conn, err := connectChild(reconnectCtx, child.ScopedServer)
	cancel()
	if err != nil {
		p.logger.Error("failed to reconnect Claude MCP server", "scope", child.Scope, "server", child.Name, "operation", operation, "error", redactSensitive(err.Error()))
		return err
	}
	oldSession, oldCancel := child.replaceConnection(conn)
	if oldSession != nil {
		_ = oldSession.Close()
	}
	if oldCancel != nil {
		oldCancel()
	}
	p.logger.Info("reconnected Claude MCP server", "scope", child.Scope, "server", child.Name, "operation", operation)
	return nil
}

func retryChildOperationOnce[T any](ctx context.Context, p *proxyServer, child *childServer, operation string, run func(context.Context, *mcpsdk.ClientSession) (T, error)) (T, error) {
	res, err := run(ctx, child.currentSession())
	if err == nil || !isChildConnectionLifecycleError(err) {
		return res, err
	}
	if reconnectErr := p.reconnectChild(ctx, child, operation); reconnectErr != nil {
		var zero T
		return zero, fmt.Errorf("%w; reconnect failed: %v", err, reconnectErr)
	}
	return run(ctx, child.currentSession())
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
				return retryChildOperationOnce(ctx, p, route.child, "completion/complete", func(ctx context.Context, session *mcpsdk.ClientSession) (*mcpsdk.CompleteResult, error) {
					return session.Complete(ctx, &params)
				})
			case "ref/resource":
				child := p.resource[params.Ref.URI]
				if child == nil {
					return nil, fmt.Errorf("unknown resource completion reference %q", params.Ref.URI)
				}
				return retryChildOperationOnce(ctx, p, child, "completion/complete", func(ctx context.Context, session *mcpsdk.ClientSession) (*mcpsdk.CompleteResult, error) {
					return session.Complete(ctx, &params)
				})
			default:
				return nil, fmt.Errorf("unsupported completion reference type %q", params.Ref.Type)
			}
		},
		SubscribeHandler: func(ctx context.Context, req *mcpsdk.SubscribeRequest) error {
			child := p.resource[req.Params.URI]
			if child == nil {
				return fmt.Errorf("unknown resource subscription %q", req.Params.URI)
			}
			_, err := retryChildOperationOnce(ctx, p, child, "resources/subscribe", func(ctx context.Context, session *mcpsdk.ClientSession) (struct{}, error) {
				return struct{}{}, session.Subscribe(ctx, req.Params)
			})
			return err
		},
		UnsubscribeHandler: func(ctx context.Context, req *mcpsdk.UnsubscribeRequest) error {
			child := p.resource[req.Params.URI]
			if child == nil {
				return fmt.Errorf("unknown resource subscription %q", req.Params.URI)
			}
			_, err := retryChildOperationOnce(ctx, p, child, "resources/unsubscribe", func(ctx context.Context, session *mcpsdk.ClientSession) (struct{}, error) {
				return struct{}{}, session.Unsubscribe(ctx, req.Params)
			})
			return err
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
		tools, err := p.collectChildTools(toolsCtx, child)
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
		prompts, err := p.collectChildPrompts(promptsCtx, child)
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
				callCtx, cancel := p.operationContext(ctx)
				defer cancel()
				res, err := route.child.currentSession().CallTool(callCtx, &mcpsdk.CallToolParams{Name: route.name, Arguments: req.Params.Arguments})
				if err != nil {
					if isChildConnectionLifecycleError(err) {
						if reconnectErr := p.reconnectChild(ctx, route.child, "tools/call"); reconnectErr != nil {
							return nil, childOperationError(route.child.ScopedServer, "tools/call", fmt.Sprintf("%q via exposed tool %q", route.name, req.Params.Name), fmt.Errorf("%w; reconnect failed: %v", err, reconnectErr))
						}
						return nil, childOperationError(route.child.ScopedServer, "tools/call", fmt.Sprintf("%q via exposed tool %q", route.name, req.Params.Name), fmt.Errorf("%w; bridge reconnected child for future calls; not retrying current tools/call because delivery state is ambiguous", err))
					}
					return nil, childOperationError(route.child.ScopedServer, "tools/call", fmt.Sprintf("%q via exposed tool %q", route.name, req.Params.Name), err)
				}
				return res, nil
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
				return retryChildOperationOnce(ctx, p, route.child, "prompts/get", func(ctx context.Context, session *mcpsdk.ClientSession) (*mcpsdk.GetPromptResult, error) {
					return session.GetPrompt(ctx, &params)
				})
			})
		}
	}

	for _, child := range p.children {
		resourcesCtx, cancelResources := p.operationContext(ctx)
		resources, err := p.collectChildResources(resourcesCtx, child)
		cancelResources()
		if err == nil {
			for _, childResource := range resources {
				resource := *childResource
				resourceChild := child
				p.resource[resource.URI] = resourceChild
				srv.AddResource(&resource, func(ctx context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
					return retryChildOperationOnce(ctx, p, resourceChild, "resources/read", func(ctx context.Context, session *mcpsdk.ClientSession) (*mcpsdk.ReadResourceResult, error) {
						return session.ReadResource(ctx, req.Params)
					})
				})
			}
		} else {
			p.logger.Debug("child resources unavailable", "server", child.Name, "error", redactSensitive(err.Error()))
		}

		templatesCtx, cancelTemplates := p.operationContext(ctx)
		templates, err := p.collectChildResourceTemplates(templatesCtx, child)
		cancelTemplates()
		if err == nil {
			for _, childTemplate := range templates {
				template := *childTemplate
				templateChild := child
				p.templateRoutes = append(p.templateRoutes, templateChild)
				srv.AddResourceTemplate(&template, func(ctx context.Context, req *mcpsdk.ReadResourceRequest) (*mcpsdk.ReadResourceResult, error) {
					return retryChildOperationOnce(ctx, p, templateChild, "resources/read", func(ctx context.Context, session *mcpsdk.ClientSession) (*mcpsdk.ReadResourceResult, error) {
						return session.ReadResource(ctx, req.Params)
					})
				})
			}
		} else {
			p.logger.Debug("child resource templates unavailable", "server", child.Name, "error", redactSensitive(err.Error()))
		}
	}

	return nil
}

func childOperationError(server ScopedServer, operation, detail string, err error) error {
	message := redactSensitive(err.Error())
	hint := childOperationFailureHint(message)
	if detail != "" {
		operation = operation + " " + detail
	}
	if hint != "" {
		return fmt.Errorf("%s-scope MCP server %q %s failed: %s (%s)", server.Scope, server.Name, operation, message, hint)
	}
	return fmt.Errorf("%s-scope MCP server %q %s failed: %s", server.Scope, server.Name, operation, message)
}

func childOperationFailureHint(message string) string {
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "missing env var"):
		return "set the referenced variable before launching cwc so Codex forwards it to claude-bridge"
	case strings.Contains(lower, "context deadline exceeded") || strings.Contains(lower, "timeout") || strings.Contains(lower, "timed out"):
		return "child MCP operation timed out; check remote server latency or raise CLAUDE_BRIDGE_OPERATION_TIMEOUT before launching cwc"
	case strings.Contains(lower, "unauthorized") || strings.Contains(lower, "status 401") || strings.Contains(lower, "status code 401") || strings.Contains(lower, "forbidden") || strings.Contains(lower, "status 403") || strings.Contains(lower, "status code 403"):
		return "check the child MCP server auth token or Authorization/header environment forwarded to claude-bridge"
	case isChildConnectionLifecycleMessage(lower):
		return "child MCP connection closed; claude-bridge reconnects the affected child for future calls, so restart Codex only if reconnect keeps failing"
	default:
		return ""
	}
}

func isChildConnectionLifecycleError(err error) bool {
	if err == nil {
		return false
	}
	return isChildConnectionLifecycleMessage(strings.ToLower(err.Error()))
}

func isChildConnectionLifecycleMessage(lower string) bool {
	return strings.Contains(lower, "eof") ||
		strings.Contains(lower, "connection closed") ||
		strings.Contains(lower, "client is closing") ||
		strings.Contains(lower, "session not found") ||
		strings.Contains(lower, "use of closed network connection") ||
		strings.Contains(lower, "broken pipe")
}

func (p *proxyServer) inspectTools(ctx context.Context) ([]exposedTool, []childFailure) {
	childTools := map[*childServer][]*mcpsdk.Tool{}
	var toolNames []string
	var failures []childFailure
	for _, child := range p.children {
		toolsCtx, cancel := p.operationContext(ctx)
		tools, err := p.collectChildTools(toolsCtx, child)
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

func (p *proxyServer) collectChildTools(ctx context.Context, child *childServer) ([]*mcpsdk.Tool, error) {
	return retryChildOperationOnce(ctx, p, child, "tools/list", func(ctx context.Context, session *mcpsdk.ClientSession) ([]*mcpsdk.Tool, error) {
		return collectTools(ctx, session)
	})
}

func (p *proxyServer) collectChildPrompts(ctx context.Context, child *childServer) ([]*mcpsdk.Prompt, error) {
	return retryChildOperationOnce(ctx, p, child, "prompts/list", func(ctx context.Context, session *mcpsdk.ClientSession) ([]*mcpsdk.Prompt, error) {
		return collectPrompts(ctx, session)
	})
}

func (p *proxyServer) collectChildResources(ctx context.Context, child *childServer) ([]*mcpsdk.Resource, error) {
	return retryChildOperationOnce(ctx, p, child, "resources/list", func(ctx context.Context, session *mcpsdk.ClientSession) ([]*mcpsdk.Resource, error) {
		return collectResources(ctx, session)
	})
}

func (p *proxyServer) collectChildResourceTemplates(ctx context.Context, child *childServer) ([]*mcpsdk.ResourceTemplate, error) {
	return retryChildOperationOnce(ctx, p, child, "resources/templates/list", func(ctx context.Context, session *mcpsdk.ClientSession) ([]*mcpsdk.ResourceTemplate, error) {
		return collectResourceTemplates(ctx, session)
	})
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
			child.close()
		}(child)
	}
	wg.Wait()
}
