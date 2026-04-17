package ordermcp

import (
	"context"

	"livepass/services/order-rpc/internal/svc"

	gomcp "github.com/zeromicro/go-zero/mcp"
)

type Server struct {
	server    gomcp.McpServer
	svcCtx    *svc.ServiceContext
	toolNames []string
}

func NewServer(c gomcp.McpConf, svcCtx *svc.ServiceContext) *Server {
	server := &Server{
		server: gomcp.NewMcpServer(c),
		svcCtx: svcCtx,
	}
	registerTools(server)
	return server
}

func (s *Server) Start() {
	s.server.Start()
}

func (s *Server) Stop() {
	s.server.Stop()
}

func (s *Server) ToolNames() []string {
	names := make([]string, len(s.toolNames))
	copy(names, s.toolNames)
	return names
}

func (s *Server) recordTool(name string) {
	s.toolNames = append(s.toolNames, name)
}

func (s *Server) makeJSONResult(ctx context.Context, payload any) (*gomcp.CallToolResult, error) {
	text, err := marshalPayload(payload)
	if err != nil {
		return nil, err
	}
	return &gomcp.CallToolResult{
		Content: []gomcp.Content{
			&gomcp.TextContent{Text: text},
		},
	}, nil
}
