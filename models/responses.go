package models

type ServerResponse struct {
	Success    bool           `json:"success"`
	StatusCode int            `json:"status_code"`
	Message    string         `json:"message"`
	Data       map[string]any `json:"data,omitempty"`
	Error      any            `json:"error,omitempty"`
}