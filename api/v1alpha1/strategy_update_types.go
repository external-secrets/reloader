package v1alpha1

type UpdateStrategy struct {
	Operation UpdateStrategyOperation `json:"operation"`
	// Required if Operation == Patch
	PatchOperationConfig *PatchOperationConfig `json:"patchOperationConfig,omitempty"`
}

type PatchOperationConfig struct {
	Path     string `json:"path"`
	Template string `json:"template"`
}

type UpdateStrategyOperation string

const (
	UpdateStrategyOperationPatchStatus UpdateStrategyOperation = "PatchStatus"
	UpdateStrategyOperationPatch       UpdateStrategyOperation = "Patch"
	UpdateStrategyOperationDelete      UpdateStrategyOperation = "Delete"
)
