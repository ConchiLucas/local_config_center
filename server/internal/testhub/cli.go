package testhub

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

func (h *Hub) runCLI(ctx context.Context, live *liveRun, binding Binding) error {
	_ = ctx
	item := binding.CLI
	if item == nil {
		return fmt.Errorf("缺少本地 CLI 配置")
	}
	live.setStage("检查可执行文件")
	h.emit(live, RunEvent{Type: "status", RunID: live.run.ID, State: StateRunning, StageLabel: "检查可执行文件"})
	path := item.Command
	info, err := os.Stat(path)
	checks := []DiagnosticItem{
		{Label: "命令路径", Status: "passed", Detail: path},
		{Label: "工作目录", Status: "not_run", Detail: empty(item.WorkingDirectory, "未指定")},
		{Label: "完整任务执行", Status: "not_run", Detail: "工作台不代跑完整 CLI 任务"},
	}
	if err != nil {
		checks[0].Status = "failed"
		checks[0].Detail = Sanitize(err.Error())
		block := OutputBlock{ID: "cli-1", Kind: "diagnostic", Checks: checks}
		live.setOutput(block)
		h.emit(live, RunEvent{Type: "output", RunID: live.run.ID, Block: &block})
		return fmt.Errorf("找不到命令文件：%s", Sanitize(err.Error()))
	}
	if info.IsDir() {
		checks = append(checks, DiagnosticItem{Label: "文件类型", Status: "failed", Detail: "路径是目录"})
		block := OutputBlock{ID: "cli-1", Kind: "diagnostic", Checks: checks}
		live.setOutput(block)
		h.emit(live, RunEvent{Type: "output", RunID: live.run.ID, Block: &block})
		return fmt.Errorf("命令路径是目录")
	}
	mode := info.Mode()
	exec := mode&0o111 != 0
	status := "passed"
	detail := fmt.Sprintf("%s · %d bytes", filepath.Base(path), info.Size())
	if !exec {
		status = "failed"
		detail += " · 当前用户不可执行"
	}
	checks = append(checks, DiagnosticItem{Label: "可执行文件", Status: status, Detail: detail})
	if item.WorkingDirectory != "" {
		if _, err := os.Stat(item.WorkingDirectory); err != nil {
			checks[1].Status = "failed"
			checks[1].Detail = Sanitize(err.Error())
		} else {
			checks[1].Status = "passed"
			checks[1].Detail = item.WorkingDirectory
		}
	}
	block := OutputBlock{ID: "cli-1", Kind: "diagnostic", Checks: checks}
	live.setOutput(block)
	h.emit(live, RunEvent{Type: "output", RunID: live.run.ID, Block: &block})
	if !exec {
		return fmt.Errorf("命令文件存在，但当前用户没有可执行权限")
	}
	return nil
}

func empty(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
