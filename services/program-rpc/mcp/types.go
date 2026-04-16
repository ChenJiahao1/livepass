package programmcp

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type PageProgramsArgs struct {
	PageNumber int64 `json:"page_number,omitempty"`
	PageSize   int64 `json:"page_size,omitempty"`
}

type GetProgramDetailArgs struct {
	ProgramID string `json:"program_id"`
}

type ProgramSummary struct {
	ProgramID string `json:"program_id"`
	Title     string `json:"title"`
	ShowTime  string `json:"show_time"`
}

type PageProgramsResult struct {
	Programs []ProgramSummary `json:"programs"`
}

type ProgramDetailResult struct {
	ProgramID string `json:"program_id"`
	Title     string `json:"title"`
	ShowTime  string `json:"show_time"`
	Place     string `json:"place"`
}

func parseProgramID(programID string) (int64, error) {
	normalized := strings.TrimSpace(programID)
	if normalized == "" {
		return 0, fmt.Errorf("invalid program_id")
	}
	id, err := strconv.ParseInt(normalized, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid program_id: %w", err)
	}
	return id, nil
}

func formatProgramID(programID int64) string {
	return strconv.FormatInt(programID, 10)
}

func marshalPayload(payload any) (string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(body), nil
}
