package domain

type Task struct {
	ID   string                 `json:"id"`
	Tool string                 `json:"tool"`
	Args map[string]interface{} `json:"args,omitempty"`
}
