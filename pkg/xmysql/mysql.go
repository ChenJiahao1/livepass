package xmysql

import (
	"database/sql"
	"time"

	"github.com/go-sql-driver/mysql"
)

const (
	DefaultMaxOpenConns    = 64
	DefaultMaxIdleConns    = 64
	DefaultConnMaxLifetime = time.Minute
	DefaultConnMaxIdleTime = time.Duration(0)
)

type Config struct {
	DataSource      string
	MaxOpenConns    int           `json:",default=64"`
	MaxIdleConns    int           `json:",default=64"`
	ConnMaxLifetime time.Duration `json:",default=1m"`
	ConnMaxIdleTime time.Duration `json:",optional"`
}

func NewConfig(dataSource string) Config {
	return Config{DataSource: dataSource}.Normalize()
}

func (c Config) Normalize() Config {
	if c.MaxOpenConns <= 0 {
		c.MaxOpenConns = DefaultMaxOpenConns
	}
	if c.MaxIdleConns <= 0 {
		c.MaxIdleConns = DefaultMaxIdleConns
	}
	if c.MaxIdleConns > c.MaxOpenConns {
		c.MaxIdleConns = c.MaxOpenConns
	}
	if c.ConnMaxLifetime <= 0 {
		c.ConnMaxLifetime = DefaultConnMaxLifetime
	}
	if c.ConnMaxIdleTime < 0 {
		c.ConnMaxIdleTime = DefaultConnMaxIdleTime
	}

	return c
}

func ApplyPool(db *sql.DB, cfg Config) {
	if db == nil {
		return
	}

	cfg = cfg.Normalize()
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
}

func WithLocalTime(dataSource string) string {
	cfg, err := mysql.ParseDSN(dataSource)
	if err != nil {
		return dataSource
	}

	cfg.Loc = time.Local

	return cfg.FormatDSN()
}
