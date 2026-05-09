package config

import (
	"fmt"
	"os"

	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig    `yaml:"server" validate:"required"`
	Cost     CostConfig      `yaml:"cost"`
	Tiers    []TierConfig    `yaml:"tiers" validate:"required,min=1"`
	Adapters []AdapterConfig `yaml:"adapters" validate:"required,min=1"`
}

type ServerConfig struct {
	Port int `yaml:"port" validate:"required,min=1,max=65535"`
}

type CostConfig struct {
	MonthlyBudgetUSD float64 `yaml:"monthly_budget_usd"`
}

type TierConfig struct {
	Name     string   `yaml:"name" validate:"required"`
	Adapters []string `yaml:"adapters" validate:"required,min=1"`
	Fallback bool     `yaml:"fallback"`
}

type AdapterConfig struct {
	Name    string `yaml:"name" validate:"required"`
	Type    string `yaml:"type" validate:"required,oneof=ollama anthropic openai_compat"`
	BaseURL string `yaml:"base_url"`
	APIKey  string `yaml:"api_key"`
	Model   string `yaml:"model" validate:"required"`
	// Optional per-token pricing in USD per million tokens. When set, overrides
	// built-in defaults for EstimateCost. Matches Anthropic's published pricing format.
	CostPerMillionInputTokens  float64 `yaml:"cost_per_million_input_tokens"`
	CostPerMillionOutputTokens float64 `yaml:"cost_per_million_output_tokens"`
}

func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	expanded := os.ExpandEnv(string(raw))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	validate := validator.New()
	if err := validate.Struct(&cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}
