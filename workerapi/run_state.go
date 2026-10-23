package workerapi

type RunState string

const (
	// RunStateUnknown is the invalid zero state of the Run.
	RunStateUnknown RunState = "UNKNOWN"
	// RunStateQueued represents a Run that hasn't been started yet.
	RunStateQueued RunState = "QUEUED"
	// RunStateCanceled means that the Run has been canceled before it started running.
	RunStateCanceled RunState = "CANCELED"
	// RunStateInitializing means that Spacelift is currently running `terraform init` and associated hooks, if any.
	RunStateInitializing RunState = "INITIALIZING"
	// RunStatePlanning means that Spacelift is currently running `terraform plan`.
	RunStatePlanning RunState = "PLANNING"
	// RunStateFailed means that the Run has failed for one reason or another.
	RunStateFailed RunState = "FAILED"
	// RunStateFinished means that the Run has finished successfully.
	RunStateFinished RunState = "FINISHED"
	// RunStateUnconfirmed means that the Run is waiting for user review and confirmation.
	RunStateUnconfirmed RunState = "UNCONFIRMED"
	// RunStateDiscarded means that the Run has been reviewed negatively by the user.
	RunStateDiscarded RunState = "DISCARDED"
	// RunStateConfirmed means that the Run has been confirmed by the user and will apply.
	RunStateConfirmed RunState = "CONFIRMED"
	// RunStateApplying means that the Run is currently applying a plan.
	RunStateApplying RunState = "APPLYING"
	// RunStatePerforming means that the Run is currently performing a task.
	RunStatePerforming RunState = "PERFORMING"
	// RunStateStopped means that the Run has been stopped by a user when it was still running.
	RunStateStopped RunState = "STOPPED"
	// RunStateDestroying means that the Run is currently deleting a plan.
	RunStateDestroying RunState = "DESTROYING"
	// RunStatePreparing means that the Run is currently being prepared by the drain and launcher to be run before planning.
	RunStatePreparing RunState = "PREPARING"
	// RunStatePreparingApply means that the Run is currently being prepared by the drain and launcher to be run before applying.
	RunStatePreparingApply RunState = "PREPARING_APPLY"
	// RunStateSkipped means that the Run has been skipped and will NOT be picked up by a worker because one of its dependencies got canceled/stopped/failed,
	// or using stack dependency references that didn't contain changes, so it's no-op.
	RunStateSkipped RunState = "SKIPPED"
	// RunStateReplanRequested means that the Run is waiting for a new plan to be created.
	RunStateReplanRequested RunState = "REPLAN_REQUESTED"
	// runStatePending is DEPRECATED AND RESERVED
	runStatePending RunState = "PENDING" //nolint:unused
	// RunStateReady represents a Run that is ready to be started.
	RunStateReady RunState = "READY"
	// RunStatePreparingReplan means that the Run is currently being prepared to be replanned.
	RunStatePreparingReplan RunState = "PREPARING_REPLAN"
	// RunStatePendingReview means that the proposed Run is currently waiting for post-planning approval policy sign-off.
	RunStatePendingReview RunState = "PENDING_REVIEW"
)
