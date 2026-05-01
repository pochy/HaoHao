package platform

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type ClickHouseConfig struct {
	Addr     string
	Database string
	Username string
	Password string
}

func NewClickHouseConn(ctx context.Context, cfg ClickHouseConfig) (driver.Conn, error) {
	addr := strings.TrimSpace(cfg.Addr)
	if addr == "" {
		return nil, fmt.Errorf("CLICKHOUSE_ADDR is required")
	}
	database := strings.TrimSpace(cfg.Database)
	if database == "" {
		database = "default"
	}
	username := strings.TrimSpace(cfg.Username)
	if username == "" {
		username = "default"
	}

	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{addr},
		Auth: clickhouse.Auth{
			Database: database,
			Username: username,
			Password: cfg.Password,
		},
		DialTimeout: 5 * time.Second,
		ReadTimeout: 10 * time.Minute,
	})
	if err != nil {
		return nil, fmt.Errorf("open clickhouse connection: %w", err)
	}
	if err := conn.Ping(ctx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ping clickhouse: %w", err)
	}
	return conn, nil
}
