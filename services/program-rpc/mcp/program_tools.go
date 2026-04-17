package programmcp

import (
	"context"

	"livepass/services/program-rpc/internal/logic"
	"livepass/services/program-rpc/pb"

	gomcp "github.com/zeromicro/go-zero/mcp"
)

func registerProgramTools(server *Server) {
	server.recordTool("page_programs")
	gomcp.AddTool(server.server, &gomcp.Tool{
		Name:        "page_programs",
		Description: "List programs for activity discovery",
	}, server.pageProgramsTool)

	server.recordTool("get_program_detail")
	gomcp.AddTool(server.server, &gomcp.Tool{
		Name:        "get_program_detail",
		Description: "Get program detail for activity service",
	}, server.getProgramDetailTool)
}

func (s *Server) PagePrograms(ctx context.Context, args PageProgramsArgs) (*PageProgramsResult, error) {
	resp, err := logic.NewPageProgramsLogic(ctx, s.svcCtx).PagePrograms(&pb.PageProgramsReq{
		PageNumber: args.PageNumber,
		PageSize:   args.PageSize,
	})
	if err != nil {
		return nil, err
	}

	result := &PageProgramsResult{
		Programs: make([]ProgramSummary, 0, len(resp.GetList())),
	}
	for _, program := range resp.GetList() {
		result.Programs = append(result.Programs, ProgramSummary{
			ProgramID: formatProgramID(program.GetId()),
			Title:     program.GetTitle(),
			ShowTime:  program.GetShowTime(),
		})
	}
	return result, nil
}

func (s *Server) pageProgramsTool(ctx context.Context, _ *gomcp.CallToolRequest, args PageProgramsArgs) (*gomcp.CallToolResult, any, error) {
	result, err := s.PagePrograms(ctx, args)
	if err != nil {
		return nil, nil, err
	}
	callResult, err := s.makeJSONResult(ctx, result)
	if err != nil {
		return nil, nil, err
	}
	return callResult, nil, nil
}

func (s *Server) GetProgramDetail(ctx context.Context, args GetProgramDetailArgs) (*ProgramDetailResult, error) {
	programID, err := parseProgramID(args.ProgramID)
	if err != nil {
		return nil, err
	}

	resp, err := logic.NewGetProgramDetailViewLogic(ctx, s.svcCtx).GetProgramDetailView(&pb.GetProgramDetailViewReq{
		Id: programID,
	})
	if err != nil {
		return nil, err
	}

	return &ProgramDetailResult{
		ProgramID: formatProgramID(resp.GetId()),
		Title:     resp.GetTitle(),
		ShowTime:  resp.GetShowTime(),
		Place:     resp.GetPlace(),
	}, nil
}

func (s *Server) getProgramDetailTool(ctx context.Context, _ *gomcp.CallToolRequest, args GetProgramDetailArgs) (*gomcp.CallToolResult, any, error) {
	result, err := s.GetProgramDetail(ctx, args)
	if err != nil {
		return nil, nil, err
	}
	callResult, err := s.makeJSONResult(ctx, result)
	if err != nil {
		return nil, nil, err
	}
	return callResult, nil, nil
}
