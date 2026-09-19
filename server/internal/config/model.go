package config

import "time"

const SchemaVersion = "1"

type AIProvider struct {
	ID           string            `json:"id"`
	Label        string            `json:"label"`
	Type         string            `json:"type"`
	BaseURL      string            `json:"baseUrl"`
	APIKey       string            `json:"apiKey"`
	Model        string            `json:"model"`
	MaxTokens    int               `json:"maxTokens"`
	Voice        string            `json:"voice"`
	Capabilities []string          `json:"capabilities"`
	Options      map[string]string `json:"options"`
	Enabled      bool              `json:"enabled"`
}

type AIConfiguration struct {
	ActiveProviderID string       `json:"activeProviderId"`
	Providers        []AIProvider `json:"providers"`
}

type DatabaseConnection struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Environment string            `json:"environment"`
	Host        string            `json:"host"`
	Port        int               `json:"port"`
	Database    string            `json:"database"`
	Username    string            `json:"username"`
	Password    string            `json:"password"`
	Parameters  map[string]string `json:"parameters"`
}

type DatabaseConfiguration struct {
	Databases []DatabaseConnection `json:"databases"`
}

type ObjectStorageConfiguration struct {
	Configured      bool   `json:"configured"`
	Enabled         bool   `json:"enabled"`
	Endpoint        string `json:"endpoint"`
	AccessKeyID     string `json:"accessKeyId"`
	SecretAccessKey string `json:"secretAccessKey"`
	UseSSL          bool   `json:"useSsl"`
	BucketName      string `json:"bucketName"`
	BasePath        string `json:"basePath"`
}

type LocalCLIConfig struct {
	ID               string   `json:"id"`
	Label            string   `json:"label"`
	Enabled          bool     `json:"enabled"`
	Command          string   `json:"command"`
	DefaultArgs      []string `json:"defaultArgs"`
	Model            string   `json:"model"`
	ReasoningEffort  string   `json:"reasoningEffort"`
	WorkingDirectory string   `json:"workingDirectory"`
	TimeoutSeconds   int      `json:"timeoutSeconds"`
	Capabilities     []string `json:"capabilities"`
}

type LocalCLIConfiguration struct {
	ActiveConfigID string           `json:"activeConfigId"`
	Configs        []LocalCLIConfig `json:"configs"`
}

type RuntimeConfiguration struct {
	SchemaVersion string                     `json:"schemaVersion"`
	GeneratedAt   time.Time                  `json:"generatedAt"`
	AI            AIConfiguration            `json:"ai"`
	Databases     []DatabaseConnection       `json:"databases"`
	ObjectStorage ObjectStorageConfiguration `json:"objectStorage"`
	LocalCLI      LocalCLIConfiguration      `json:"localCli"`
}

func CapabilitySet(values []string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, value := range values {
		key := NormalizeCapability(value)
		if key != "" {
			out[key] = true
		}
	}
	return out
}

func NormalizeCapability(value string) string {
	return trimUpper(value)
}
