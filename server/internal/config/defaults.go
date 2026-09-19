package config

import "strings"

func DefaultAI() AIConfiguration {
	return AIConfiguration{Providers: make([]AIProvider, 0)}
}

func DefaultDatabases() DatabaseConfiguration {
	return DatabaseConfiguration{Databases: make([]DatabaseConnection, 0)}
}

func DefaultObjectStorage() ObjectStorageConfiguration {
	return ObjectStorageConfiguration{}
}

func DefaultLocalCLI() LocalCLIConfiguration {
	return LocalCLIConfiguration{Configs: make([]LocalCLIConfig, 0)}
}

func NormalizeAI(value AIConfiguration) AIConfiguration {
	if value.Providers == nil {
		value.Providers = make([]AIProvider, 0)
	}
	for i := range value.Providers {
		if value.Providers[i].Capabilities == nil {
			value.Providers[i].Capabilities = make([]string, 0)
		}
		if value.Providers[i].Options == nil {
			value.Providers[i].Options = make(map[string]string)
		}
	}
	return value
}

func NormalizeDatabases(value DatabaseConfiguration) DatabaseConfiguration {
	if value.Databases == nil {
		value.Databases = make([]DatabaseConnection, 0)
	}
	for i := range value.Databases {
		if value.Databases[i].Parameters == nil {
			value.Databases[i].Parameters = make(map[string]string)
		}
	}
	return value
}

func NormalizeLocalCLI(value LocalCLIConfiguration) LocalCLIConfiguration {
	if value.Configs == nil {
		value.Configs = make([]LocalCLIConfig, 0)
	}
	for i := range value.Configs {
		if value.Configs[i].DefaultArgs == nil {
			value.Configs[i].DefaultArgs = make([]string, 0)
		}
		if value.Configs[i].Capabilities == nil {
			value.Configs[i].Capabilities = make([]string, 0)
		}
	}
	return value
}

func trimUpper(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func trimLower(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func HasCapability(provider AIProvider, name string) bool {
	want := trimUpper(name)
	for _, capability := range provider.Capabilities {
		if trimUpper(capability) == want {
			return true
		}
	}
	return false
}

func IsOpenAICompatible(provider AIProvider) bool {
	return trimLower(provider.Type) == "openai-compatible"
}
