package dto

// HealthLiveResult 是存活探针响应，仅表示进程存活。
type HealthLiveResult struct {
	Status    string `json:"status" example:"ok"`
	App       string `json:"app" example:"sdu-alumni-platform"`
	Env       string `json:"env" example:"development"`
	StartedAt string `json:"started_at" example:"2026-01-01T00:00:00+08:00"`
}

// HealthReadyResult 是就绪探针响应。
// 已启用的依赖为 connected，未启用的为 disabled。
type HealthReadyResult struct {
	Status   string `json:"status" example:"ok"`
	Database string `json:"database" enums:"connected,disabled" example:"connected"`
	Redis    string `json:"redis" enums:"connected,disabled" example:"connected"`
}
