package cli

import (
	"errors"
	"strconv"

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

// exitCodeError carries an explicit process exit code for a successful command
// that still wants to signal a condition (e.g. `cve --exit-code`: the lookup
// succeeded, but a vulnerability was found). It is not a failure, so Execute
// does not print it as an error — only the code is used.
type exitCodeError struct{ code int }

func (e exitCodeError) Error() string {
	return "requested exit code " + strconv.Itoa(e.code)
}

// ExitCodeFor maps an error to a process exit code.
func ExitCodeFor(err error) int {
	if err == nil {
		return ExitOK
	}
	var ece exitCodeError
	if errors.As(err, &ece) {
		return ece.code
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
