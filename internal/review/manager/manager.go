package manager

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"txt-cleaning/internal/config"
	"txt-cleaning/internal/processor/preprocess"
)

// ReviewStatus 审核状态
type ReviewStatus string

const (
	StatusPending   ReviewStatus = "pending"
	StatusApproved  ReviewStatus = "approved"
	StatusRejected  ReviewStatus = "rejected"
	StatusSkipped   ReviewStatus = "skipped"
)

// ReviewItem 审核项
type ReviewItem struct {
	ID           string         `json:"id"`
	Suggestion   preprocess.Change `json:"suggestion"`
	Status       ReviewStatus   `json:"status"`
	ReviewedAt   *time.Time     `json:"reviewedAt"`
	ReviewerNote string         `json:"reviewerNote"`
}

// ReviewSession 审核会话
type ReviewSession struct {
	ID           string        `json:"id"`
	FileID       string        `json:"fileId"`
	ProcessID    string        `json:"processId"`
	Items        []ReviewItem  `json:"items"`
	CreatedAt    time.Time     `json:"createdAt"`
	LastModified time.Time     `json:"lastModified"`
	Completed    bool          `json:"completed"`
}

// Manager 审核管理器
type Manager struct {
	sessions map[string]*ReviewSession
}

// NewManager 创建新的审核管理器
func NewManager() *Manager {
	return &Manager{
		sessions: make(map[string]*ReviewSession),
	}
}

// CreateSession 创建审核会话
func (m *Manager) CreateSession(sessionID, fileID, processID string, suggestions []preprocess.Change) (*ReviewSession, error) {
	// 创建审核项
	items := make([]ReviewItem, len(suggestions))
	for i, suggestion := range suggestions {
		items[i] = ReviewItem{
			ID:         string(i + 1),
			Suggestion: suggestion,
			Status:     StatusPending,
		}
	}

	// 创建会话
	session := &ReviewSession{
		ID:           sessionID,
		FileID:       fileID,
		ProcessID:    processID,
		Items:        items,
		CreatedAt:    time.Now(),
		LastModified: time.Now(),
		Completed:    false,
	}

	// 保存到内存
	m.sessions[sessionID] = session

	// 持久化
	err := m.saveSession(session)
	if err != nil {
		return nil, err
	}

	return session, nil
}

// GetSession 获取审核会话
func (m *Manager) GetSession(sessionID string) (*ReviewSession, error) {
	// 从内存中获取
	session, exists := m.sessions[sessionID]
	if exists {
		return session, nil
	}

	// 从磁盘加载
	session, err := m.loadSession(sessionID)
	if err != nil {
		return nil, err
	}

	// 保存到内存
	m.sessions[sessionID] = session

	return session, nil
}

// UpdateItemStatus 更新审核项状态
func (m *Manager) UpdateItemStatus(sessionID, itemID string, status ReviewStatus, note string) error {
	// 获取会话
	session, err := m.GetSession(sessionID)
	if err != nil {
		return err
	}

	// 更新审核项
	now := time.Now()
	for i := range session.Items {
		if session.Items[i].ID == itemID {
			session.Items[i].Status = status
			session.Items[i].ReviewedAt = &now
			session.Items[i].ReviewerNote = note
			session.LastModified = now
			break
		}
	}

	// 检查是否所有项都已审核
	session.Completed = true
	for _, item := range session.Items {
		if item.Status == StatusPending {
			session.Completed = false
			break
		}
	}

	// 持久化
	return m.saveSession(session)
}

// SaveProgress 保存审核进度
func (m *Manager) SaveProgress(sessionID string, note string) error {
	// 获取会话
	session, err := m.GetSession(sessionID)
	if err != nil {
		return err
	}

	// 更新最后修改时间
	session.LastModified = time.Now()

	// 持久化
	return m.saveSession(session)
}

// GetPendingItems 获取待审核项
func (m *Manager) GetPendingItems(sessionID string) ([]ReviewItem, error) {
	// 获取会话
	session, err := m.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	// 过滤待审核项
	pending := []ReviewItem{}
	for _, item := range session.Items {
		if item.Status == StatusPending {
			pending = append(pending, item)
		}
	}

	return pending, nil
}

// GetApprovedSuggestions 获取已批准的建议
func (m *Manager) GetApprovedSuggestions(sessionID string) ([]preprocess.Change, error) {
	// 获取会话
	session, err := m.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	// 过滤已批准的建议
	approved := []preprocess.Change{}
	for _, item := range session.Items {
		if item.Status == StatusApproved {
			approved = append(approved, item.Suggestion)
		}
	}

	return approved, nil
}

// saveSession 保存会话到磁盘
func (m *Manager) saveSession(session *ReviewSession) error {
	sessionPath := filepath.Join(config.AppConfigInstance.DataDir, "reviews", session.ID+".json")

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(sessionPath), 0755); err != nil {
		return err
	}

	// 序列化
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}

	// 写入文件
	return os.WriteFile(sessionPath, data, 0644)
}

// loadSession 从磁盘加载会话
func (m *Manager) loadSession(sessionID string) (*ReviewSession, error) {
	sessionPath := filepath.Join(config.AppConfigInstance.DataDir, "reviews", sessionID+".json")

	// 读取文件
	data, err := os.ReadFile(sessionPath)
	if err != nil {
		return nil, err
	}

	// 反序列化
	session := &ReviewSession{}
	if err := json.Unmarshal(data, session); err != nil {
		return nil, err
	}

	return session, nil
}