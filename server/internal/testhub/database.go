package testhub

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"local_config_center/internal/config"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

func (h *Hub) runDatabase(ctx context.Context, live *liveRun, binding Binding, req CreateTestRequest) error {
	database := binding.Database
	if database == nil {
		return fmt.Errorf("缺少数据库配置")
	}
	probe := strings.ToLower(strings.TrimSpace(req.Input.ProbeID))
	if probe == "" {
		probe = "connect"
	}
	if probe != "connect" && probe != "select_one" && probe != "ping" {
		return APIError{Status: 400, Code: "INVALID_INPUT", Message: "只允许 connect / select_one / ping 探针"}
	}
	engine := strings.ToLower(strings.TrimSpace(database.Type))
	host, port := h.rewriter.ApplyHostPort(database.Host, database.Port)
	if host == "169.254.169.254" {
		return fmt.Errorf("目标地址不被允许")
	}
	started := time.Now()
	live.setStage("正在执行只读探针")
	h.emit(live, RunEvent{Type: "status", RunID: live.run.ID, State: StateRunning, StageLabel: "正在执行只读探针"})

	checks := []DiagnosticItem{
		{Label: "目标服务", Status: "passed", Detail: fmt.Sprintf("%s %s:%d/%s", engine, host, port, database.Database)},
		{Label: "写入操作", Status: "not_run", Detail: "不开放"},
	}
	var result string
	var err error
	switch {
	case engine == "postgres" || engine == "postgresql" || engine == "pg":
		if probe == "ping" {
			probe = "select_one"
		}
		result, err = probePostgres(ctx, *database, host, port, probe)
		label := "连接/探针"
		if err != nil {
			checks = append(checks, DiagnosticItem{Label: label, Status: "failed", Detail: Sanitize(err.Error())})
			block := OutputBlock{ID: "db-1", Kind: "diagnostic", Checks: checks}
			live.setOutput(block)
			h.emit(live, RunEvent{Type: "output", RunID: live.run.ID, Block: &block})
			return err
		}
		checks = append(checks, DiagnosticItem{Label: label, Status: "passed", Detail: result})
	case engine == "redis":
		result, err = probeRedis(ctx, database.Password, host, port)
		if err != nil {
			checks = append(checks, DiagnosticItem{Label: "PING", Status: "failed", Detail: Sanitize(err.Error())})
			block := OutputBlock{ID: "db-1", Kind: "diagnostic", Checks: checks}
			live.setOutput(block)
			h.emit(live, RunEvent{Type: "output", RunID: live.run.ID, Block: &block})
			return err
		}
		checks = append(checks, DiagnosticItem{Label: "PING", Status: "passed", Detail: result})
	default:
		return fmt.Errorf("尚未实现 %s 的只读探针", database.Type)
	}
	elapsed := time.Since(started).Milliseconds()
	table := OutputBlock{
		ID:      "db-table",
		Kind:    "table",
		Columns: []string{"检查项", "结果"},
		Rows: [][]any{
			{"目标服务", fmt.Sprintf("%s://%s:%d/%s", engine, host, port, database.Database)},
			{"探针", probeLabel(engine, probe)},
			{"耗时", fmt.Sprintf("%d ms", elapsed)},
			{"返回", result},
			{"写入操作", "不开放"},
		},
	}
	diag := OutputBlock{ID: "db-1", Kind: "diagnostic", Checks: checks}
	live.setOutput(diag)
	live.setOutput(table)
	h.emit(live, RunEvent{Type: "output", RunID: live.run.ID, Block: &diag})
	h.emit(live, RunEvent{Type: "output", RunID: live.run.ID, Block: &table})
	return nil
}

func probeLabel(engine, probe string) string {
	if engine == "redis" {
		return "PING"
	}
	if probe == "connect" {
		return "连接"
	}
	return "SELECT 1"
}

func probePostgres(ctx context.Context, database config.DatabaseConnection, host string, port int, probe string) (string, error) {
	sslmode := "disable"
	if database.Parameters != nil {
		if v := strings.TrimSpace(database.Parameters["sslmode"]); v != "" {
			sslmode = v
		}
	}
	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(database.Username, database.Password),
		Host:   net.JoinHostPort(host, strconv.Itoa(port)),
		Path:   "/" + database.Database,
	}
	q := url.Values{}
	q.Set("sslmode", sslmode)
	q.Set("connect_timeout", "5")
	u.RawQuery = q.Encode()
	dialCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	conn, err := pgx.Connect(dialCtx, u.String())
	if err != nil {
		return "", fmt.Errorf("连接失败: %s", Sanitize(err.Error()))
	}
	defer conn.Close(context.Background())
	if probe == "connect" {
		if err := conn.Ping(dialCtx); err != nil {
			return "", err
		}
		return "连接成功", nil
	}
	var one int
	if err := conn.QueryRow(dialCtx, "SELECT 1").Scan(&one); err != nil {
		return "", err
	}
	return fmt.Sprintf("SELECT 1 → %d", one), nil
}

func probeRedis(ctx context.Context, password, host string, port int) (string, error) {
	client := redis.NewClient(&redis.Options{
		Addr:        net.JoinHostPort(host, strconv.Itoa(port)),
		Password:    password,
		DialTimeout: 5 * time.Second,
		ReadTimeout: 5 * time.Second,
	})
	defer client.Close()
	dialCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	res, err := client.Ping(dialCtx).Result()
	if err != nil {
		return "", err
	}
	return res, nil
}
