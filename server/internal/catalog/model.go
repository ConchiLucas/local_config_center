package catalog

import "time"

type Catalog struct {
	Source        string    `json:"source"`
	SchemaVersion string    `json:"schemaVersion"`
	GeneratedAt   time.Time `json:"generatedAt"`
	Groups        []Group   `json:"groups"`
}

type Group struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Eyebrow  string `json:"eyebrow"`
	Mark     string `json:"mark"`
	Icon     string `json:"icon"`
	Pages    []Page `json:"pages"`
	SortRank int    `json:"-"`
}

type Page struct {
	ID          string         `json:"id"`
	Label       string         `json:"label"`
	En          string         `json:"en"`
	Kind        string         `json:"kind"`
	Icon        string         `json:"icon"`
	Description string         `json:"description"`
	Configs     []PublicConfig `json:"configs"`
}

type Property struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type CapabilitySet struct {
	Verified         bool     `json:"verified"`
	Delivery         []string `json:"delivery,omitempty"`
	Formats          []string `json:"formats,omitempty"`
	ReferenceImage   bool     `json:"referenceImage"`
	Cancellation     string   `json:"cancellation"`
	AcceptedInput    []string `json:"acceptedInputMimeTypes,omitempty"`
	MaxInputBytes    int      `json:"maxInputBytes,omitempty"`
}

type Verification struct {
	At       string `json:"at"`
	Revision string `json:"revision"`
	Status   string `json:"status"`
}

type ModelOption struct {
	ProviderID string `json:"providerId,omitempty"`
	Label      string `json:"label"`
	Model      string `json:"model"`
	Category   string `json:"category,omitempty"`
	Note       string `json:"note,omitempty"`
	Registered bool   `json:"registered,omitempty"`
}

type PickerOption struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type ModelFamilySpec struct {
	ID          string         `json:"id"`
	Label       string         `json:"label"`
	Model       string         `json:"model,omitempty"`
	Base        string         `json:"base,omitempty"`
	Resolutions []PickerOption `json:"resolutions,omitempty"`
	Aspects     []PickerOption `json:"aspects,omitempty"`
}

type ModelPicker struct {
	Style    string            `json:"style,omitempty"`
	Families []ModelFamilySpec `json:"families,omitempty"`
}

type PublicConfig struct {
	ID               string         `json:"id"`
	GroupID          string         `json:"groupId"`
	PageID           string         `json:"pageId"`
	Kind             string         `json:"kind"`
	Title            string         `json:"title"`
	Model            string         `json:"model"`
	Revision         string         `json:"revision"`
	RendererKey      string         `json:"rendererKey"`
	PublicProperties []Property     `json:"publicProperties"`
	Properties       [][]string     `json:"properties"`
	CredentialState  string         `json:"credentialState"`
	Capabilities     CapabilitySet  `json:"capabilities"`
	LastVerification *Verification  `json:"lastVerification,omitempty"`
	Probe            string         `json:"probe,omitempty"`
	Status           string         `json:"status"`
	Models           []ModelOption  `json:"models,omitempty"`
	MemberIDs        []string       `json:"memberIds,omitempty"`
	AllowCustomModel bool           `json:"allowCustomModel,omitempty"`
	ModelPicker      ModelPicker    `json:"modelPicker,omitempty"`
}

type Family struct {
	ID      string
	Label   string
	Eyebrow string
	Mark    string
	Icon    string
	Rank    int
}
