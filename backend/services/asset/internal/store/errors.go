package store

import "errors"

// Refusals the store decides itself, inside the transaction that would have
// made the change, so a caller can tell them apart from a database failure.
var (
	// ErrNotFound: the asset or request does not exist (or the asset is deleted).
	ErrNotFound = errors.New("not found")
	// ErrAssetUnavailable: the asset is not in a state that can be handed over,
	// or somebody else holds it.
	ErrAssetUnavailable = errors.New("asset is not available")
	// ErrWrongType: the asset is not of the type (or workspace) the request is for.
	ErrWrongType = errors.New("asset is not of the requested type")
	// ErrNotAMember: the person to receive the asset does not belong to its workspace.
	ErrNotAMember = errors.New("person is not a member of the workspace")
	// ErrRequestNotOpen: the request is not in the status this step needs.
	ErrRequestNotOpen = errors.New("request is not open for this step")
	// ErrSameHolder: the asset is already held by the person named.
	ErrSameHolder = errors.New("asset is already held by this person")
	// ErrStateChanged: the asset is not in the state (or held by the person) the
	// caller decided on; it changed, or was deleted, since it was read.
	ErrStateChanged = errors.New("asset changed since it was read")
)
