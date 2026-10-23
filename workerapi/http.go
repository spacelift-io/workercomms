package workerapi

// Paths use the same {placeholder} form as pollcomms.URLAck, so the server
// routes on them as they are and clients substitute the ULID.
const (
	URLRunState     = "/worker/run_state"
	URLIntentResult = "/worker/intent-tasks/{taskID}/result"
	URLScanEvents   = "/worker/scan-runs/{scanRunULID}/events"
)
