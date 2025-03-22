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
	Info(ctx context.Context, mod, ver string) (string, time.Time, map[string]json.RawMessage, error)
	Latest(ctx context.Context, mod string) (string, time.Time, map[string]json.RawMessage, error)
	List(context.Context, string) ([]string, error)
	Mod(ctx context.Context, mod, ver string) (io.ReadCloser, error)
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
