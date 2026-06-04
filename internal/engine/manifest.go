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

// OptionalGroup is presentation metadata for an optional component (one id may
// span several files). The UI renders one toggle per group; the engine itself
// only needs ManifestFile.Optional/ID for sync filtering.
type OptionalGroup struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Desc      string `json:"desc,omitempty"`
	DefaultOn bool   `json:"defaultOn,omitempty"`
}

type Manifest struct {
	Files            []ManifestFile  `json:"files"`
	Optional         []OptionalGroup `json:"optional,omitempty"`
	StrictDirs       []string        `json:"strictDirs"`
	Runtime          string          `json:"runtime"`
	RecommendedRAMMB int             `json:"recommendedRamMB"`
	MinRAMMB         int             `json:"minRamMB"`
}
