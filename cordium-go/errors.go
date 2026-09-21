/*
 * Copyright Octelium Labs, LLC. All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package cordium

import (
	"context"
	"errors"
	"fmt"

	"github.com/octelium/octelium/apis/main/cordiumv1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	// ErrClientClosed is returned once the Client has been closed.
	ErrClientClosed = errors.New("cordium: client is closed")
	// ErrInvalidArgument reports an argument that the SDK rejected locally.
	ErrInvalidArgument = errors.New("cordium: invalid argument")
	// ErrWorkspaceDeleted is returned by the wait helpers when the Workspace
	// they are following is deleted.
	ErrWorkspaceDeleted = errors.New("cordium: Workspace was deleted")
	// ErrExecNotRunning is returned by the ExecSession operations that need a
	// live command (e.g. writing to its standard input) once it has exited.
	ErrExecNotRunning = errors.New("cordium: exec session is no longer running")
	// ErrTerminalClosed is returned once a Terminal has been closed.
	ErrTerminalClosed = errors.New("cordium: terminal is closed")
)

func invalidArgumentf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidArgument, fmt.Sprintf(format, args...))
}

// ExitError reports that a command executed inside a Workspace exited with a
// non-zero status code. [Exec] does not return it on its own: it is produced by
// [ExecResult.Err] so that callers keep full access to the captured output.
type ExitError struct {
	// Command is the command that was executed.
	Command string
	// ExitCode is the status code with which the command exited.
	ExitCode int
	// Stderr is the captured standard error, when the result captured it.
	Stderr []byte
}

func (e *ExitError) Error() string {
	if e == nil {
		return "cordium: command exited with a non-zero status code"
	}
	ret := fmt.Sprintf("cordium: command %q exited with code %d", e.Command, e.ExitCode)
	if len(e.Stderr) > 0 {
		ret += ": " + truncateForError(string(e.Stderr))
	}
	return ret
}

// WorkspaceFailureError reports that a Workspace run failed. It carries the
// Cluster's own structured failure so that callers can branch on the exact
// reason (e.g. a failed image build or a failed lifecycle Task).
type WorkspaceFailureError struct {
	// Workspace is the name of the Workspace whose run failed.
	Workspace string
	// Failure is the structured failure reported by the Cluster. It may be nil
	// when the Workspace stopped unexpectedly without reporting one.
	Failure *cordiumv1.Workspace_Status_Failure
}

func (e *WorkspaceFailureError) Error() string {
	if e == nil {
		return "cordium: Workspace run failed"
	}
	ret := fmt.Sprintf("cordium: Workspace %q failed", e.Workspace)
	if reason := FailureReason(e.Failure); reason != "" {
		ret += ": " + reason
	}
	if msg := e.Failure.GetMessage(); msg != "" {
		ret += ": " + truncateForError(msg)
	}
	return ret
}

// FailureReason returns a short, stable, human readable identifier of the
// reason of a Workspace run failure (e.g. "ImageBuild" or "Task"). It returns
// an empty string when the failure is nil or carries no specific reason.
func FailureReason(failure *cordiumv1.Workspace_Status_Failure) string {
	if failure == nil {
		return ""
	}

	switch failure.GetType().(type) {
	case *cordiumv1.Workspace_Status_Failure_ImageBuild_:
		return "ImageBuild"
	case *cordiumv1.Workspace_Status_Failure_ImagePull_:
		return "ImagePull"
	case *cordiumv1.Workspace_Status_Failure_RepoClone_:
		return "RepoClone"
	case *cordiumv1.Workspace_Status_Failure_AdditionalRepoClone_:
		return "AdditionalRepoClone"
	case *cordiumv1.Workspace_Status_Failure_BuildTimeoutExceeded_:
		return "BuildTimeoutExceeded"
	case *cordiumv1.Workspace_Status_Failure_Task_:
		return "Task"
	case *cordiumv1.Workspace_Status_Failure_StartupUnknown_:
		return "StartupUnknown"
	case *cordiumv1.Workspace_Status_Failure_StartupTimeoutExceeded_:
		return "StartupTimeoutExceeded"
	case *cordiumv1.Workspace_Status_Failure_LoadStorage_:
		return "LoadStorage"
	case *cordiumv1.Workspace_Status_Failure_SaveStorage_:
		return "SaveStorage"
	case *cordiumv1.Workspace_Status_Failure_StoppageTimeoutExceeded_:
		return "StoppageTimeoutExceeded"
	case *cordiumv1.Workspace_Status_Failure_RunContainer_:
		return "RunContainer"
	case *cordiumv1.Workspace_Status_Failure_HealthCheck_:
		return "HealthCheck"
	case *cordiumv1.Workspace_Status_Failure_NetworkPolicy_:
		return "NetworkPolicy"
	case *cordiumv1.Workspace_Status_Failure_Unknown_:
		return "Unknown"
	default:
		return ""
	}
}

// SnapshotFailureError reports that a WorkspaceSnapshot could not be taken. It
// carries the Cluster's own structured failure so that callers can branch on
// the exact reason.
type SnapshotFailureError struct {
	// Snapshot is the WorkspaceSnapshot that failed.
	Snapshot *cordiumv1.WorkspaceSnapshot
}

func (e *SnapshotFailureError) Error() string {
	if e == nil {
		return "cordium: WorkspaceSnapshot failed"
	}
	ret := fmt.Sprintf("cordium: WorkspaceSnapshot %q failed",
		e.Snapshot.GetMetadata().GetName())
	if reason := SnapshotFailureReason(e.Snapshot.GetStatus().GetFailure()); reason != "" {
		ret += ": " + reason
	}
	if msg := e.Snapshot.GetStatus().GetFailure().GetMessage(); msg != "" {
		ret += ": " + truncateForError(msg)
	}
	return ret
}

// SnapshotFailureReason returns a short, stable, human readable identifier of
// the reason of a WorkspaceSnapshot failure (e.g. "Storage" or "Unsupported").
// It returns an empty string when the failure is nil or carries no specific
// reason.
func SnapshotFailureReason(failure *cordiumv1.WorkspaceSnapshot_Status_Failure) string {
	if failure == nil {
		return ""
	}

	switch failure.GetType().(type) {
	case *cordiumv1.WorkspaceSnapshot_Status_Failure_Unsupported_:
		return "Unsupported"
	case *cordiumv1.WorkspaceSnapshot_Status_Failure_SourceNotFound_:
		return "SourceNotFound"
	case *cordiumv1.WorkspaceSnapshot_Status_Failure_Storage_:
		return "Storage"
	case *cordiumv1.WorkspaceSnapshot_Status_Failure_Unknown_:
		return "Unknown"
	default:
		return ""
	}
}

// VolumeFailureError reports that the underlying storage of a Volume could not
// be provisioned or that it was lost. It carries the Cluster's own structured
// failure so that callers can branch on the exact reason.
type VolumeFailureError struct {
	// Volume is the Volume that failed.
	Volume *cordiumv1.Volume
}

func (e *VolumeFailureError) Error() string {
	if e == nil {
		return "cordium: Volume failed"
	}
	ret := fmt.Sprintf("cordium: Volume %q failed", e.Volume.GetMetadata().GetName())
	if reason := VolumeFailureReason(e.Volume.GetStatus().GetFailure()); reason != "" {
		ret += ": " + reason
	}
	if msg := e.Volume.GetStatus().GetFailure().GetMessage(); msg != "" {
		ret += ": " + truncateForError(msg)
	}
	return ret
}

// VolumeFailureReason returns a short, stable, human readable identifier of the
// reason of a Volume failure (e.g. "Storage" or "Unsupported"). It returns an
// empty string when the failure is nil or carries no specific reason.
func VolumeFailureReason(failure *cordiumv1.Volume_Status_Failure) string {
	if failure == nil {
		return ""
	}

	switch failure.GetType().(type) {
	case *cordiumv1.Volume_Status_Failure_Unsupported_:
		return "Unsupported"
	case *cordiumv1.Volume_Status_Failure_Storage_:
		return "Storage"
	case *cordiumv1.Volume_Status_Failure_Unknown_:
		return "Unknown"
	default:
		return ""
	}
}

// IsNotFound reports whether err is a Cluster NOT_FOUND error.
func IsNotFound(err error) bool { return hasCode(err, codes.NotFound) }

// IsAlreadyExists reports whether err is a Cluster ALREADY_EXISTS error.
func IsAlreadyExists(err error) bool { return hasCode(err, codes.AlreadyExists) }

// IsInvalidArgument reports whether err is a Cluster INVALID_ARGUMENT error or
// an argument that the SDK itself rejected.
func IsInvalidArgument(err error) bool {
	return errors.Is(err, ErrInvalidArgument) || hasCode(err, codes.InvalidArgument)
}

// IsPermissionDenied reports whether err is a Cluster PERMISSION_DENIED error.
func IsPermissionDenied(err error) bool { return hasCode(err, codes.PermissionDenied) }

// IsUnauthenticated reports whether err is a Cluster UNAUTHENTICATED error.
func IsUnauthenticated(err error) bool { return hasCode(err, codes.Unauthenticated) }

// IsResourceExhausted reports whether err is a Cluster RESOURCE_EXHAUSTED error
// (e.g. a Cluster or Space quota was reached).
func IsResourceExhausted(err error) bool { return hasCode(err, codes.ResourceExhausted) }

// IsUnavailable reports whether err is a Cluster UNAVAILABLE error. Such errors
// are typically transient and worth retrying.
func IsUnavailable(err error) bool { return hasCode(err, codes.Unavailable) }

// IsCanceled reports whether err is a cancellation of the caller's context.
func IsCanceled(err error) bool {
	return errors.Is(err, context.Canceled) || hasCode(err, codes.Canceled)
}

// IsDeadlineExceeded reports whether err is a deadline expiration.
func IsDeadlineExceeded(err error) bool {
	return errors.Is(err, context.DeadlineExceeded) || hasCode(err, codes.DeadlineExceeded)
}

// Code returns the gRPC status code of a Cluster error. It returns
// [codes.Unknown] for the errors that did not originate from the Cluster.
func Code(err error) codes.Code {
	if err == nil {
		return codes.OK
	}
	return status.Code(err)
}

func hasCode(err error, code codes.Code) bool {
	if err == nil {
		return false
	}
	if s, ok := status.FromError(err); ok {
		return s.Code() == code
	}
	// status.FromError does not unwrap, so give wrapped Cluster errors a chance.
	var se interface{ GRPCStatus() *status.Status }
	if errors.As(err, &se) {
		return se.GRPCStatus().Code() == code
	}
	return false
}

func truncateForError(s string) string {
	const max = 512
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
