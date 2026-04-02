package status

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// PipelineStatus is persisted to .chainagent/status/<reqID>.json.
type PipelineStatus struct {
	ReqID          string `json:"req_id"`
	ChangeName     string `json:"change_name"`
	Title          string `json:"title,omitempty"`
	Phase          string `json:"phase"`           // planning|development|testing|fixing|done|failed
	ManagerStatus  string `json:"manager_status"`  // pending|in_progress|completed|failed
	PipelineStatus string `json:"pipeline_status"` // in_progress|completed|failed
	UpdatedAt      string `json:"updated_at"`
}

// LiveState is persisted to .chainagent/live/<reqID>/<agent>.json.
type LiveState struct {
	Agent        string `json:"agent"`
	ReqID        string `json:"req_id"`
	CurrentTool  string `json:"current_tool,omitempty"`
	StepCount    int    `json:"step_count"`
	LastActivity string `json:"last_activity"`
	Model        string `json:"model,omitempty"`
	Title        string `json:"title,omitempty"`
}

func statusDir(root string) string {
	return filepath.Join(root, ".chainagent", "status")
}

func liveDir(root, reqID string) string {
	return filepath.Join(root, ".chainagent", "live", reqID)
}

// Read loads the status for a given req. Returns nil if not found.
func Read(root, reqID string) (*PipelineStatus, error) {
	path := filepath.Join(statusDir(root), reqID+".json")
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var s PipelineStatus
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parsing status %q: %w", path, err)
	}
	return &s, nil
}

// Write persists the status for a given req.
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

// CreateInitial builds a fresh PipelineStatus and writes it.
func CreateInitial(root, reqID, changeName, title string) (*PipelineStatus, error) {
	s := &PipelineStatus{
		ReqID:          reqID,
		ChangeName:     changeName,
		Title:          title,
		Phase:          "init",
		ManagerStatus:  "pending",
		PipelineStatus: "in_progress",
	}
	if err := Write(root, reqID, s); err != nil {
		return nil, err
	}
	return s, nil
}

// WriteLive updates the live state file for an agent.
func WriteLive(root, reqID, agent string, state *LiveState) error {
	dir := liveDir(root, reqID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	state.Agent = agent
	state.ReqID = reqID
	state.LastActivity = time.Now().Format(time.RFC3339)
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, agent+".json"), data, 0o644)
}

// ListAll returns all statuses found in the status directory.
func ListAll(root string) ([]*PipelineStatus, error) {
	dir := statusDir(root)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var result []*PipelineStatus
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		reqID := e.Name()[:len(e.Name())-5]
		s, err := Read(root, reqID)
		if err != nil || s == nil {
			continue
		}
		result = append(result, s)
	}
	return result, nil
}
