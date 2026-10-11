package model

import "time"

// IdentityChange is what happened to a reader's account at the identity
// provider; see docs/design/accounts.md section 1.
type IdentityChange string

const (
	IdentityRemoved     IdentityChange = "removed"
	IdentityDeactivated IdentityChange = "deactivated"
	IdentityReactivated IdentityChange = "reactivated"
	IdentityLocked      IdentityChange = "locked"
	IdentityUnlocked    IdentityChange = "unlocked"
)

// IdentityEvent is one such change of the account the provider knows by
// Subject. ID is unique per event, for refusing a delivery seen before.
type IdentityEvent struct {
	ID      string
	Subject string
	Change  IdentityChange
	At      time.Time
}
