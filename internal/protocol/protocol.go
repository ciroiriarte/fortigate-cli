// Package protocol models the FortiOS REST API response envelope and error
// shape, and decodes them into typed values for the rest of the CLI.
//
// A FortiOS API response is a JSON object like:
//
//	{
//	  "http_method": "GET",
//	  "results": [ ... ] | { ... },
//	  "vdom": "root",
//	  "path": "firewall",
//	  "name": "address",
//	  "status": "success",
//	  "http_status": 200,
//	  "serial": "FGT...",
//	  "version": "v7.6.0"
//	}
//
// The payload lives under "results"; everything else is metadata.
package protocol

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Kind categorizes an APIError for exit-code mapping.
type Kind string

const (
	KindTransport Kind = "transport" // network/TLS failure, no HTTP response
	KindAuth      Kind = "auth"      // 401/403
	KindNotFound  Kind = "not_found" // 404 / 424
	KindConflict  Kind = "conflict"  // 4xx validation/duplicate
	KindServer    Kind = "server"    // 5xx
	KindAPI       Kind = "api"       // other non-2xx
)

// APIError is a decoded FortiOS error (or a transport failure).
type APIError struct {
	Kind       Kind
	HTTPStatus int
	// Code is the FortiOS numeric "error" field when present.
	Code    int
	Message string
}

func (e *APIError) Error() string {
	if e.HTTPStatus > 0 {
		return fmt.Sprintf("%s (http %d)", e.Message, e.HTTPStatus)
	}
	return e.Message
}

// Envelope is the standard FortiOS response wrapper.
type Envelope struct {
	Results    json.RawMessage `json:"results"`
	Status     string          `json:"status"`
	HTTPStatus int             `json:"http_status"`
	VDOM       string          `json:"vdom"`
	Serial     string          `json:"serial"`
	Version    string          `json:"version"`
	Revision   string          `json:"revision"`
	// Mkey echoes the primary key on single-object and write (create/update)
	// responses. It can decode from a JSON string or number, so it is captured
	// raw and read via MkeyString.
	Mkey     json.RawMessage `json:"mkey"`
	Error    int             `json:"error"`
	CLIError string          `json:"cli_error"`
}

// MkeyString returns the response mkey as a string, unquoting a JSON string and
// stringifying a JSON number. Empty when absent.
func (e *Envelope) MkeyString() string {
	s := strings.TrimSpace(string(e.Mkey))
	if s == "" || s == "null" {
		return ""
	}
	if len(s) >= 2 && s[0] == '"' {
		var out string
		if err := json.Unmarshal(e.Mkey, &out); err == nil {
			return out
		}
	}
	return strings.Trim(s, `"`)
}

// DecodeEnvelope unmarshals just the response wrapper (no payload typing).
func DecodeEnvelope(body []byte) (*Envelope, error) {
	var env Envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, err
	}
	return &env, nil
}

// DecodeData unmarshals a success response body into out. It extracts the
// "results" field when present, and otherwise decodes the whole body, so it
// works for cmdb objects, monitor endpoints, and the odd action that returns a
// bare object.
func DecodeData(body []byte, out any) error {
	if out == nil {
		return nil
	}
	var env Envelope
	if err := json.Unmarshal(body, &env); err == nil && len(env.Results) > 0 {
		return json.Unmarshal(env.Results, out)
	}
	return json.Unmarshal(body, out)
}

// DecodeError builds an APIError from a non-2xx response.
// errorCodeText maps the common FortiOS numeric `error` codes to a short
// message, used when the body carries no cli_error. Not exhaustive (~300+
// exist); an unmapped code is surfaced in the generic fallback message and
// retained on APIError.Code.
var errorCodeText = map[int]string{
	-3:  "entry not found",
	-5:  "unable to match the entry",
	-8:  "duplicate entry",
	-14: "permission denied (access profile)",
	-23: "invalid value for a field",
}

func DecodeError(status int, body []byte) *APIError {
	e := &APIError{Kind: kindForStatus(status), HTTPStatus: status}
	var env Envelope
	if err := json.Unmarshal(body, &env); err == nil {
		e.Code = env.Error
		switch {
		case env.CLIError != "":
			e.Message = env.CLIError
		case errorCodeText[env.Error] != "":
			e.Message = errorCodeText[env.Error]
		case status == 403:
			// A bare 403 is ambiguous — auth/CSRF vs an access-profile or
			// VDOM-scope permission problem. Disambiguate for the user.
			e.Message = "permission denied (403) — the account's access profile may lack rights for this operation, a VDOM-scope mismatch, or a missing/expired session (CSRF)"
		case env.Status != "" && env.Status != "success":
			e.Message = fmt.Sprintf("FortiOS returned status %q", env.Status)
		}
	}
	if e.Message == "" {
		e.Message = fmt.Sprintf("request failed with HTTP %d", status)
	}
	// Surface an unmapped numeric code so it stays diagnosable (mapped codes and
	// cli_error already carry meaning, so only append when the message is generic).
	if e.Code != 0 && (strings.HasPrefix(e.Message, "request failed with HTTP") || strings.HasPrefix(e.Message, "FortiOS returned status")) {
		e.Message = fmt.Sprintf("%s (error %d)", e.Message, e.Code)
	}
	return e
}

// IsNotFound reports whether err is a FortiOS "entry not found" — HTTP 404/424
// or numeric error -3. Used by the --upsert fallback.
func IsNotFound(err error) bool {
	var ae *APIError
	if !errors.As(err, &ae) {
		return false
	}
	return ae.Kind == KindNotFound || ae.Code == -3
}

func kindForStatus(status int) Kind {
	switch {
	case status == 401, status == 403:
		return KindAuth
	case status == 404, status == 424:
		return KindNotFound
	case status >= 500:
		return KindServer
	case status >= 400:
		return KindConflict
	default:
		return KindAPI
	}
}
