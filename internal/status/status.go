// Package status 负责读写流水线的持久化状态。
//
// 状态文件分两类：
//   - 流水线状态：.chainagent/status/<reqID>.json   — 记录当前阶段、整体进度
//   - 实时状态：  .chainagent/live/<reqID>/<角色>.json — 记录 Agent 正在执行的工具步骤
package status

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// PipelineStatus 描述一个需求（REQ）的整体流水线进度。
// 持久化到 .chainagent/status/<reqID>.json。
type PipelineStatus struct {
	ReqID          string `json:"req_id"`          // 需求 ID，如 "001"
	ChangeName     string `json:"change_name"`     // OpenSpec change 名称，如 "req-001"
	Title          string `json:"title,omitempty"` // 需求标题（可选）
	Phase          string `json:"phase"`           // 当前阶段：init|planning|planning_done|development|testing-N|fixing-N|completed|fix_failed|failed
	ManagerStatus  string `json:"manager_status"`  // Manager Agent 状态：pending|in_progress|completed|failed
	PipelineStatus string `json:"pipeline_status"` // 整体流水线状态：in_progress|completed|failed
	UpdatedAt      string `json:"updated_at"`      // 最后更新时间（RFC3339 格式）
}

// LiveState 描述某个 Agent 当前正在执行的工具调用状态。
// 持久化到 .chainagent/live/<reqID>/<agent>.json，供实时监控使用。
type LiveState struct {
	Agent        string `json:"agent"`                  // Agent 角色名，如 "frontend"
	ReqID        string `json:"req_id"`                 // 关联的需求 ID
	CurrentTool  string `json:"current_tool,omitempty"` // 当前正在调用的工具名，如 "bash"
	StepCount    int    `json:"step_count"`             // 已执行的工具步骤总数
	LastActivity string `json:"last_activity"`          // 最后活动时间（RFC3339 格式）
	Model        string `json:"model,omitempty"`        // 使用的模型名称
	Title        string `json:"title,omitempty"`        // 任务标题
}

// statusDir 返回流水线状态文件的存放目录。
func statusDir(root string) string {
	return filepath.Join(root, ".chainagent", "status")
}

// liveDir 返回某个需求下所有 Agent 实时状态文件的存放目录。
func liveDir(root, reqID string) string {
	return filepath.Join(root, ".chainagent", "live", reqID)
}

// Read 读取指定需求的流水线状态。
// 若状态文件不存在，返回 (nil, nil)；其他错误则返回 (nil, err)。
func Read(root, reqID string) (*PipelineStatus, error) {
	path := filepath.Join(statusDir(root), reqID+".json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil // 文件不存在视为正常（尚未创建）
	}
	if err != nil {
		return nil, err
	}
	var s PipelineStatus
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("解析状态文件 %q 失败: %w", path, err)
	}
	return &s, nil
}

// Write 将流水线状态持久化到磁盘，同时自动更新 UpdatedAt 时间戳。
func Write(root, reqID string, s *PipelineStatus) error {
	dir := statusDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	s.UpdatedAt = time.Now().Format(time.RFC3339)
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, reqID+".json"), data, 0o644)
}

// CreateInitial 构建初始状态并写入磁盘。
// 在流水线第一次启动时调用，防止后续步骤读取到 nil。
func CreateInitial(root, reqID, changeName, title string) (*PipelineStatus, error) {
	s := &PipelineStatus{
		ReqID:          reqID,
		ChangeName:     changeName,
		Title:          title,
		Phase:          "init",        // 初始阶段
		ManagerStatus:  "pending",     // Manager 尚未启动
		PipelineStatus: "in_progress", // 整体流水线已启动
	}
	if err := Write(root, reqID, s); err != nil {
		return nil, err
	}
	return s, nil
}

// WriteLive 更新指定 Agent 的实时状态文件。
// 每次 Agent 调用一个新工具时触发，供 `chainagent status` 实时展示。
func WriteLive(root, reqID, agent string, state *LiveState) error {
	dir := liveDir(root, reqID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	// 自动填充关联字段，调用方无需手动设置。
	state.Agent = agent
	state.ReqID = reqID
	state.LastActivity = time.Now().Format(time.RFC3339)
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, agent+".json"), data, 0o644)
}

// ListAll 读取状态目录下所有 .json 文件并返回对应的流水线状态列表。
// 解析失败或文件为空的条目会被跳过，不影响其他记录的读取。
func ListAll(root string) ([]*PipelineStatus, error) {
	dir := statusDir(root)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil // 目录不存在说明还没有任何需求运行过
	}
	if err != nil {
		return nil, err
	}
	var result []*PipelineStatus
	for _, e := range entries {
		// 只处理 .json 文件，跳过目录和其他文件。
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		// 文件名格式为 "<reqID>.json"，去掉后缀即为 reqID。
		reqID := e.Name()[:len(e.Name())-5]
		s, err := Read(root, reqID)
		if err != nil || s == nil {
			continue // 解析失败则跳过，不影响其他记录
		}
		result = append(result, s)
	}
	return result, nil
}
