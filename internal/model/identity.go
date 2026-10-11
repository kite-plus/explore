package model

import "time"

// IdentityState is what the identity provider says of a reader's account
// now; see docs/design/accounts.md section 1.
type IdentityState string

const (
	IdentityActive      IdentityState = "active"
	IdentityDeactivated IdentityState = "deactivated"
	IdentityLocked      IdentityState = "locked"
	IdentityRemoved     IdentityState = "removed"
)

// IdentityEvent is a change the provider recorded, or pushed, for the
// account it knows by Subject. It says which account to look at; what to do
// follows from the account's state, read from the provider. ID is unique
// per event; Type is the provider's, such as "user.locked".
type IdentityEvent struct {
	ID      string
	Subject string
	Type    string
	At      time.Time
}

// RemovalEvent is the event type of an account's deletion. Explore deletes
// an account only when such an event and the provider's state agree.
const RemovalEvent = "user.removed"
