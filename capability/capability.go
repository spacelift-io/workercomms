// Package capability names the features that a launcher or a kube-workerpool-controller build
// supports.
package capability

// Capability names a feature that a launcher or a kube-workerpool-controller build supports.
// Spacelift must not send work that needs a capability to a worker whose build did not report it:
// an old binary cannot refuse work that it does not understand.
//
// The component that registers the worker reports the list of its own build: the launcher on a
// VM, the controller on Kubernetes. Both report the same name for the same feature, so Spacelift
// checks one name in both setups.
type Capability string
