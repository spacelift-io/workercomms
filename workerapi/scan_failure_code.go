package workerapi

// ScanFailureCode says why a scan run, or one of its resource types, failed.
// The numbers are stored in the database and sent by workers that may be older
// than the backend, so never reorder or reuse them: only add new codes at the end.
type ScanFailureCode uint

const (
	ScanFailureCodeUnknown ScanFailureCode = iota
	ScanFailureCodeInternal
	ScanFailureCodeManual
	ScanFailureCodeInvalidConfig
	ScanFailureCodeCredentialsAssumeFailed
	ScanFailureCodeDispatchFailed
	ScanFailureCodeAssignmentUndelivered
	ScanFailureCodeAssignmentUnstarted
	ScanFailureCodeWorkerUnresponsive
	ScanFailureCodeWorkerCrashed
	ScanFailureCodeWorkerKilled
	ScanFailureCodeLauncherSetup
	ScanFailureCodeProviderDownload
	ScanFailureCodeProviderConfigInvalid
	ScanFailureCodeProviderConfigure
	ScanFailureCodeUnsupportedResource
	ScanFailureCodeDependencySkipped
	ScanFailureCodeTimeout
	ScanFailureCodeCredentialsExpired
	ScanFailureCodeAccessDenied
	ScanFailureCodeThrottled
	ScanFailureCodeInvalidCredentials
	ScanFailureCodeServiceNotEnabled
	ScanFailureCodeProviderError
)

//exhaustive:enforce
//nolint:gosec // G101: false positive, these are failure code names, not credentials
var scanFailureCodeNames = map[ScanFailureCode]string{
	ScanFailureCodeUnknown:                 "UNKNOWN",
	ScanFailureCodeInternal:                "INTERNAL",
	ScanFailureCodeManual:                  "MANUAL",
	ScanFailureCodeInvalidConfig:           "INVALID_CONFIG",
	ScanFailureCodeCredentialsAssumeFailed: "CREDENTIALS_ASSUME_FAILED",
	ScanFailureCodeDispatchFailed:          "DISPATCH_FAILED",
	ScanFailureCodeAssignmentUndelivered:   "ASSIGNMENT_UNDELIVERED",
	ScanFailureCodeAssignmentUnstarted:     "ASSIGNMENT_UNSTARTED",
	ScanFailureCodeWorkerUnresponsive:      "WORKER_UNRESPONSIVE",
	ScanFailureCodeWorkerCrashed:           "WORKER_CRASHED",
	ScanFailureCodeWorkerKilled:            "WORKER_KILLED",
	ScanFailureCodeLauncherSetup:           "LAUNCHER_SETUP",
	ScanFailureCodeProviderDownload:        "PROVIDER_DOWNLOAD",
	ScanFailureCodeProviderConfigInvalid:   "PROVIDER_CONFIG_INVALID",
	ScanFailureCodeProviderConfigure:       "PROVIDER_CONFIGURE",
	ScanFailureCodeUnsupportedResource:     "UNSUPPORTED_RESOURCE",
	ScanFailureCodeDependencySkipped:       "DEPENDENCY_SKIPPED",
	ScanFailureCodeTimeout:                 "TIMEOUT",
	ScanFailureCodeCredentialsExpired:      "CREDENTIALS_EXPIRED",
	ScanFailureCodeAccessDenied:            "ACCESS_DENIED",
	ScanFailureCodeThrottled:               "THROTTLED",
	ScanFailureCodeInvalidCredentials:      "INVALID_CREDENTIALS",
	ScanFailureCodeServiceNotEnabled:       "SERVICE_NOT_ENABLED",
	ScanFailureCodeProviderError:           "PROVIDER_ERROR",
}

func (c ScanFailureCode) Known() bool {
	_, ok := scanFailureCodeNames[c]
	return ok
}

func (c ScanFailureCode) String() string {
	if name, ok := scanFailureCodeNames[c]; ok {
		return name
	}
	return scanFailureCodeNames[ScanFailureCodeUnknown]
}
