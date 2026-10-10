// Package reason holds the machine-readable reasons the drive service gives
// when it refuses a request that is well formed and allowed but cannot be done.
// They travel as the message of a gRPC FailedPrecondition and as the `reason`
// field of the REST refusal (409, or 413 for a full quota), so a screen can say
// what to do without parsing prose.
package reason

// FolderHasDocuments: a folder cannot be permanently deleted while it, or a
// folder beneath it, still holds text documents.
const FolderHasDocuments = "folder_has_documents"

// QuotaExceeded: the file would take the workspace past its storage quota.
const QuotaExceeded = "quota_exceeded"

// ItemChanged: another request changed the item first; the caller may retry.
const ItemChanged = "item_changed"
