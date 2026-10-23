package workload

// Set by the Kubernetes controller on the gRPC sidecar.
const (
	EnvWorkerCommsProtocol = "SPACELIFT_WORKER_COMMS_PROTOCOL"
	EnvK8sMessageFile      = "SPACELIFT_K8S_MESSAGE_FILE"
)
