package model

import "fmt"

func ProgramCacheKey(programID int64) string {
	return fmt.Sprintf("%s%v", cacheDProgramIdPrefix, programID)
}

func ProgramGroupCacheKey(programGroupID int64) string {
	return fmt.Sprintf("%s%v", cacheDProgramGroupIdPrefix, programGroupID)
}

func ProgramFirstShowTimeCacheKey(programID int64) string {
	return fmt.Sprintf("cache:dProgramShowTime:first:programId:%d", programID)
}
