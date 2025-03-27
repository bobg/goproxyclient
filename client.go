// Package goproxyclient provides a client for talking to Go module proxies.
package goproxyclient

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bobg/mid"
)

// Client is a client for talking to a sequence of one or more Go module proxies.
// Create one with [New].
type Client struct {
	first single
	rest  []nextSingle
}

type nextSingle struct {
	client      single
	afterAnyErr bool
}

// New creates a new [Client] talking to a sequence of one or more Go module proxies.
//
// The proxies are specified as described for the GOPROXY environment variable at https://go.dev/ref/mod#goproxy-protocol.
// That string specifies a list of proxy URLs separated by commas (,) or pipes (|).
// For each query type, the proxies are tried in sequence,
// and the result of the first successful request is returned.
//
// If a request is unsuccessful, then whether the next proxy in the sequence is tried
// depends on whether a comma or a pipe introduces the next proxy in the sequence:
//
//   - If a comma, then the next proxy is tried only if the failure is a 404 (Not Found) or 410 (Gone) error.
//   - If a pipe, then the next proxy is tried regardless of the failure.
//
// This function ignores any entry in the input string that is "direct", "off", or empty.
// If no proxy URL is found, it uses https://proxy.golang.org by default.
//
// If hc is non-nil, it will use that HTTP client for all requests,
// otherwise it will use a default HTTP client
// (but a distinct one from [http.DefaultClient]).
func New(goproxy string, hc *http.Client) Client {
	var (
		first       single
		afterAnyErr bool
	)
	for {
		end := strings.IndexFunc(goproxy, func(r rune) bool { return r == ',' || r == '|' })
		if end < 0 {
			switch goproxy {
			case "direct", "off", "":
				return Client{first: newSingle("https://proxy.golang.org", hc)}
			}
			return Client{first: newSingle(goproxy, hc)}
		}
		part := goproxy[:end]
		switch part {
		case "direct", "off", "":
			goproxy = goproxy[end+1:]
			continue
		}
		first = newSingle(part, hc)
		afterAnyErr = goproxy[end] == '|'
		goproxy = goproxy[end+1:]
		break
	}

	var rest []nextSingle

	for goproxy != "" {
		end := strings.IndexFunc(goproxy, func(r rune) bool { return r == ',' || r == '|' })
		if end < 0 {
			switch goproxy {
			case "direct", "off", "":
				// do nothing
			default:
				rest = append(rest, nextSingle{client: newSingle(goproxy, hc), afterAnyErr: afterAnyErr})
			}
			break
		}

		part := goproxy[:end]
		switch part {
		case "direct", "off", "":
			afterAnyErr = goproxy[end] == '|'
			goproxy = goproxy[end+1:]
			continue
		}
		rest = append(rest, nextSingle{client: newSingle(part, hc), afterAnyErr: afterAnyErr})
		afterAnyErr = goproxy[end] == '|'
		goproxy = goproxy[end+1:]
	}

	return Client{first: first, rest: rest}
}

func (cl Client) loop(errptr *error, f func(single)) {
	f(cl.first)
	if *errptr == nil {
		return
	}
	for _, next := range cl.rest {
		if !next.afterAnyErr {
			if !IsNotFound(*errptr) {
				return
			}
		}
		f(next.client) // will update *errptr
		if *errptr == nil {
			return
		}
	}
}

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
func (cl Client) Info(ctx context.Context, mod, ver string) (string, time.Time, map[string]json.RawMessage, error) {
	var (
		canonicalVer string
		tm           time.Time
		j            map[string]json.RawMessage
		err          error
	)

	cl.loop(&err, func(s single) {
		canonicalVer, tm, j, err = s.info(ctx, mod, ver)
	})

	return canonicalVer, tm, j, err
}

// Latest gets info about the latest version of a Go module.
// Its return values are the same as for [Client.Info].
func (cl Client) Latest(ctx context.Context, mod string) (string, time.Time, map[string]json.RawMessage, error) {
	var (
		canonicalVer string
		tm           time.Time
		j            map[string]json.RawMessage
		err          error
	)

	cl.loop(&err, func(s single) {
		canonicalVer, tm, j, err = s.latest(ctx, mod)
	})

	return canonicalVer, tm, j, err
}

// List lists the available versions of a Go module.
// The result is sorted in semver order
// (see [semver.Sort]).
func (cl Client) List(ctx context.Context, mod string) ([]string, error) {
	var (
		versions []string
		err      error
	)

	cl.loop(&err, func(s single) {
		versions, err = s.list(ctx, mod)
	})

	return versions, err
}

// Mod gets the go.mod file for a specific version of a Go module.
func (cl Client) Mod(ctx context.Context, mod, ver string) (io.ReadCloser, error) {
	var (
		rc  io.ReadCloser
		err error
	)

	cl.loop(&err, func(s single) {
		rc, err = s.mod(ctx, mod, ver)
	})

	return rc, err
}

// Zip gets the contents of a specific version of a Go module as a zip file.
func (cl Client) Zip(ctx context.Context, mod, ver string) (io.ReadCloser, error) {
	var (
		rc  io.ReadCloser
		err error
	)

	cl.loop(&err, func(s single) {
		rc, err = s.zip(ctx, mod, ver)
	})

	return rc, err
}

// CodeErr is the type of an error that has an associated HTTP status code.
// This interface is satisfied by [mid.CodeErr] from github.com/bobg/mid.
type CodeErr interface {
	error
	Code() int
}

// Ensure we're using a recent-enough version of the mid package.
var _ CodeErr = mid.CodeErr{}

// IsNotFound tests an error to see if it is a [CodeErr] and has status code 404 (Not Found) or 410 (Gone).
func IsNotFound(err error) bool {
	var codeErr CodeErr
	if !errors.As(err, &codeErr) {
		return false
	}
	code := codeErr.Code()
	return code == http.StatusNotFound || code == http.StatusGone
}
