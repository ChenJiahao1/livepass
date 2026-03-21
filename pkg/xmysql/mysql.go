package xmysql

import (
	"time"

	"github.com/go-sql-driver/mysql"
)

type Config struct {
	DataSource string
}

func NewConfig(dataSource string) Config {
	return Config{DataSource: dataSource}
}

func WithLocalTime(dataSource string) string {
	cfg, err := mysql.ParseDSN(dataSource)
	if err != nil {
		return dataSource
	}

	cfg.Loc = time.Local

	return cfg.FormatDSN()
}
