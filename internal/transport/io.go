package transport

import (
	"io"
	"os"
)

// stderr is the sink for --debug request/response logging. It is a package
// variable so tests can capture it.
var stderr io.Writer = os.Stderr
