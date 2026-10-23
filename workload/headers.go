package workload

import "time"

// The headers are the fields of each assignment payload that a launcher reads
// before handing the payload to the worker. Backend's full payload types embed
// them untagged, so a header must never get its own MarshalJSON or
// UnmarshalJSON: the promoted method would encode the whole payload as just
// the header.

type RunHeader struct {
	ID                string   `json:"id"`
	RunULID           string   `json:"run_ulid"`
	DockerImage       string   `json:"docker_image"`
	LauncherImage     string   `json:"launcher_image"`
	SpaceliftAPIToken string   `json:"spacelift_api_token"`
	SpaceSlug         string   `json:"space_slug,omitempty"`
	StackSlug         string   `json:"stack_slug,omitempty"`
	StackLabels       []string `json:"stack_labels,omitempty"`
}

type IntentHeader struct {
	TaskID            string `json:"task_id"`
	DockerImage       string `json:"docker_image"`
	LauncherImage     string `json:"launcher_image"`
	TimeoutSeconds    int    `json:"timeout_seconds"`
	SpaceliftAPIToken string `json:"spacelift_api_token"`
}

type ScanHeader struct {
	ScanULID          string `json:"scan_ulid"`
	ScanRunULID       string `json:"scan_run_ulid"`
	DockerImage       string `json:"docker_image"`
	LauncherImage     string `json:"launcher_image"`
	TimeoutSeconds    int    `json:"timeout_seconds"`
	SpaceliftAPIToken string `json:"spacelift_api_token"`
}

const ScanDefaultTimeout = 4 * 60 * 60 * time.Second // We reap after 4 hours.
