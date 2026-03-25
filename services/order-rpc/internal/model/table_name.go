package model

import "strings"

func normalizeTableName(table string) string {
	if table == "" {
		return table
	}
	if strings.HasPrefix(table, "`") && strings.HasSuffix(table, "`") {
		return table
	}

	return "`" + table + "`"
}

func rawTableName(table string) string {
	return strings.Trim(table, "`")
}
