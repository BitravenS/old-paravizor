package ai

// TODO: Add support for oauth (claude or chatgpt subscriptions)

type AIConfig struct {
	Enabled     bool   `yaml:"enabled,omitempty" validate:"omitempty"`
	Provider    string `yaml:"provider,omitempty" validate:"omitempty"`
	Model       string `yaml:"model,omitempty" validate:"omitempty"`
	APIKey      string `yaml:"api_key,omitempty" validate:"omitempty"`
	BaseURL     string `yaml:"base_url,omitempty" validate:"omitempty,url"`
	ConsentMode string `yaml:"consent_mode,omitempty" validate:"omitempty,oneof=always_ask auto_approve"`
}
