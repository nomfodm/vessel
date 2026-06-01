package engine

type ManifestFile struct {
	Path     string `json:"path"`
	SHA256   string `json:"sha256"`
	Size     int64  `json:"size"`
	Optional bool   `json:"optional"`
	// ID identifies an optional file so config.enabledOptional can reference it
	// (e.g. "optifine"). Empty for required files.
	ID string `json:"id,omitempty"`
}

type Manifest struct {
	Files            []ManifestFile `json:"files"`
	StrictDirs       []string       `json:"strictDirs"`
	Runtime          string         `json:"runtime"`
	RecommendedRAMMB int            `json:"recommendedRamMB"`
	MinRAMMB         int            `json:"minRamMB"`
}
