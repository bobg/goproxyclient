// Package goproxyclient provides a client for talking to Go module proxies.
package goproxyclient

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
)

// Client is the type of a client for a Go module proxy, or a sequence of them.
// See [New] and [NewMulti].
//
// A method encountering an HTTP error is required to return a [CodeErr] with the HTTP status code.
type Client interface {
	// Info gets information about a specific version of a Go module.
	// A Go module proxy produces a JSON object with Version and Time fields,
	// and possibly others.
	//
	// This function returns the canonical version string, the timestamp for that version,
	// and a map of all the fields parsed from the JSON object.
	//
	// The returned version may be different from the one supplied as an argument,
	// which is not required to be canonical.
	// (It may be a branch name or commit hash, for example.)
	//
	// The values in the map are unparsed JSON that can be further decoded with calls to [json.Unmarshal].
	Info(ctx context.Context, mod, ver string) (string, time.Time, map[string]json.RawMessage, error)

	// Latest gets info about the latest version of a Go module.
	// Its return values are the same as for [Client.Info].
	Latest(ctx context.Context, mod string) (string, time.Time, map[string]json.RawMessage, error)

	// List lists the available versions of a Go module.
	// The result is sorted in semver order.
	List(context.Context, string) ([]string, error)

	// Mod gets the go.mod file for a specific version of a Go module.
	Mod(ctx context.Context, mod, ver string) (io.ReadCloser, error)

	// Zip gets the contents of a specific version of a Go module as a zip file.
	Zip(ctx context.Context, mod, ver string) (io.ReadCloser, error)
}

// CodeErr is the type of an error that has an associated HTTP status code.
// This interface is satisfied by [mid.CodeErr] from github.com/bobg/mid.
type CodeErr interface {
	error
	Code() int
}

// IsNotFound tests an error to see if it is a [CodeErr] and has status code 404 (Not Found) or 410 (Gone).
func IsNotFound(err error) bool {
	var codeErr CodeErr
	if !errors.As(err, &codeErr) {
		return false
	}
	code := codeErr.Code()
	return code == http.StatusNotFound || code == http.StatusGone
}
