package domain

type Result struct {
	TaskID  string                 `json:"task_id"`
	Status  string                 `json:"status"`
	Success bool                   `json:"success"`
	Data    map[string]interface{} `json:"data,omitempty"`
	Error   string                 `json:"error,omitempty"`
}