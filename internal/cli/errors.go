package cli

import (
	"errors"

	"github.com/ciroiriarte/fortigate-cli/internal/protocol"
)

// Exit codes. 0 = success; the rest give scripts a coarse failure taxonomy.
const (
	ExitOK        = 0
	ExitError     = 1 // generic
	ExitUsage     = 2 // bad flags/args (cobra also uses this)
	ExitAuth      = 3 // 401/403
	ExitNotFound  = 4 // 404
	ExitConflict  = 5 // 4xx validation/duplicate
	ExitServer    = 6 // 5xx
	ExitTransport = 7 // network/TLS
	ExitCanceled  = 8 // user declined a confirmation
)

// errCanceled is returned when the user declines a destructive confirmation.
var errCanceled = errors.New("aborted")

// ExitCodeFor maps an error to a process exit code.
func ExitCodeFor(err error) int {
	if err == nil {
		return ExitOK
	}
	if errors.Is(err, errCanceled) {
		return ExitCanceled
	}
	var apiErr *protocol.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.Kind {
		case protocol.KindAuth:
			return ExitAuth
		case protocol.KindNotFound:
			return ExitNotFound
		case protocol.KindConflict:
			return ExitConflict
		case protocol.KindServer:
			return ExitServer
		case protocol.KindTransport:
			return ExitTransport
		}
	}
	return ExitError
}
