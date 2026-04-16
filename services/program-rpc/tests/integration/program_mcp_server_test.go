package integration_test

import (
	"context"
	"testing"

	programmcp "damai-go/services/program-rpc/mcp"

	gomcp "github.com/zeromicro/go-zero/mcp"
)

func TestProgramMCPServer_ListsTools(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)

	server := programmcp.NewServer(gomcp.McpConf{}, svcCtx)

	toolNames := server.ToolNames()
	if len(toolNames) != 2 {
		t.Fatalf("expected 2 tools, got %d (%v)", len(toolNames), toolNames)
	}
	if toolNames[0] != "page_programs" || toolNames[1] != "get_program_detail" {
		t.Fatalf("unexpected tool names: %v", toolNames)
	}
}

func TestProgramMCPPagePrograms(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)

	server := programmcp.NewServer(gomcp.McpConf{}, svcCtx)
	result, err := server.PagePrograms(context.Background(), programmcp.PageProgramsArgs{
		PageNumber: 1,
		PageSize:   10,
	})
	if err != nil {
		t.Fatalf("PagePrograms returned error: %v", err)
	}
	if len(result.Programs) == 0 {
		t.Fatalf("expected programs, got %+v", result)
	}
	first := result.Programs[0]
	if first.ProgramID != "10001" || first.Title != "Phase1 示例演出" || first.ShowTime != "2026-12-31 19:30:00" {
		t.Fatalf("unexpected first program: %+v", first)
	}
}

func TestProgramMCPGetProgramDetail(t *testing.T) {
	svcCtx := newProgramTestServiceContext(t)
	resetProgramDomainState(t)

	server := programmcp.NewServer(gomcp.McpConf{}, svcCtx)
	result, err := server.GetProgramDetail(context.Background(), programmcp.GetProgramDetailArgs{
		ProgramID: "10001",
	})
	if err != nil {
		t.Fatalf("GetProgramDetail returned error: %v", err)
	}
	if result.ProgramID != "10001" {
		t.Fatalf("expected program_id 10001, got %+v", result)
	}
	if result.Title != "Phase1 示例演出" || result.ShowTime != "2026-12-31 19:30:00" || result.Place != "北京示例剧场" {
		t.Fatalf("unexpected program detail: %+v", result)
	}
}
