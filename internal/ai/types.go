package ai

// TODO: Add support for oauth (claude or chatgpt subscriptions)

type AIConfig struct {
	Enabled     bool   `yaml:"enabled" validate:"boolean"`
	Provider    string `yaml:"provider"`
	Model       string `yaml:"model"`
	APIKey      string `yaml:"api_key"`
	BaseURL     string `yaml:"base_url" validate:"omitempty,url"`
	ConsentMode string `yaml:"consent_mode" validate:"omitempty,oneof=always_ask auto_approve"`
}

type ChatMessage struct {
	Role    string
	Content string
}
