// Package goproxyclient provides a client for talking to Go module proxies.
package goproxyclient

import (
	"context"
	"encoding/json"
	"io"
	"iter"
	"net/http"
	"strings"
	"time"

	"github.com/bobg/errors"
	"github.com/bobg/mid"
	"golang.org/x/mod/module"
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
// It calls [Parse] on the input string to get the sequence of proxies,
// ignoring any "direct," "off," or empty entries.
// If no proxies are specified,
// it uses https://proxy.golang.org by default.
//
// If hc is non-nil, it will use that HTTP client for all requests,
// otherwise it will use a default HTTP client
// (but a distinct one from [http.DefaultClient]).
func New(goproxy string, hc *http.Client) Client {
	seq := Parse(goproxy)
	next, stop := iter.Pull2(seq)
	defer stop()

	var first single

	for {
		val, _, ok := next()
		if !ok {
			return Client{first: newSingle("https://proxy.golang.org", hc)}
		}
		switch val {
		case "direct", "off", "":
			continue
		}
		first = newSingle(val, hc)
		break
	}

	var rest []nextSingle

	for {
		val, afterAnyErr, ok := next()
		if !ok {
			break
		}
		switch val {
		case "direct", "off", "":
			continue
		}
		rest = append(rest, nextSingle{
			client:      newSingle(val, hc),
			afterAnyErr: afterAnyErr,
		})
	}

	return Client{first: first, rest: rest}
}

// Parse parses a GOPROXY string structured as described at https://go.dev/ref/mod#goproxy-protocol:
// a sequence of strings separated by commas (,) or pipes (|).
// The strings are URLs to use in Go module proxy queries,
// or the special strings "direct" or "off".
// Comma means "try the next proxy only if the previous one failed with a 404 (Not Found) or 410 (Gone) error."
// Pipe means "try the next proxy regardless of the previous error."
//
// The result is a sequence of pairs:
// the string, and boolean meaning "after any error"
// (i.e., whether the preceding separator was a pipe).
// The boolean is false for the first element in the sequence.
func Parse(goproxy string) iter.Seq2[string, bool] {
	return func(yield func(string, bool) bool) {
		var afterAnyErr bool
		for {
			end := strings.IndexFunc(goproxy, func(r rune) bool { return r == ',' || r == '|' })
			if end < 0 {
				yield(goproxy, afterAnyErr)
				return
			}
			part := goproxy[:end]
			if !yield(part, afterAnyErr) {
				return
			}
			afterAnyErr = goproxy[end] == '|'
			goproxy = goproxy[end+1:]
		}
	}
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

	mod, err = module.EscapePath(mod)
	if err != nil {
		return "", tm, nil, errors.Wrap(err, "escaping module path")
	}
	ver, err = module.EscapeVersion(ver)
	if err != nil {
		return "", tm, nil, errors.Wrap(err, "escaping module version")
	}

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

	mod, err = module.EscapePath(mod)
	if err != nil {
		return "", tm, nil, errors.Wrap(err, "escaping module path")
	}

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

	mod, err = module.EscapePath(mod)
	if err != nil {
		return nil, errors.Wrap(err, "escaping module path")
	}

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

	mod, err = module.EscapePath(mod)
	if err != nil {
		return nil, errors.Wrap(err, "escaping module path")
	}
	ver, err = module.EscapeVersion(ver)
	if err != nil {
		return nil, errors.Wrap(err, "escaping module version")
	}

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

	mod, err = module.EscapePath(mod)
	if err != nil {
		return nil, errors.Wrap(err, "escaping module path")
	}
	ver, err = module.EscapeVersion(ver)
	if err != nil {
		return nil, errors.Wrap(err, "escaping module version")
	}

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
