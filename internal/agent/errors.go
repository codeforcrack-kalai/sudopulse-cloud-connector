package agent

import "errors"

// ErrNoToken is returned when the agent has no saved state and no install
// token was provided for registration.
var ErrNoToken = errors.New("no install token provided and no existing state found")
