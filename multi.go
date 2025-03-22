package goproxyclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type multi struct {
	first Client
	rest  []nextClient
}

type nextClient struct {
	client      Client
	afterAnyErr bool
}

var _ Client = multi{}

// NewMulti creates a new [Client] talking to a possible sequence of Go module proxies.
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
//
// If hc is non-nil, it will use that HTTP client for all requests,
// otherwise it will use a default client
// (a distinct one from [http.DefaultClient]).
func NewMulti(goproxy string, hc *http.Client) (Client, error) {
	return newMulti(goproxy, hc, New)
}

func newMulti(goproxy string, hc *http.Client, newClient func(string, *http.Client) Client) (Client, error) {
	var (
		first       Client
		afterAnyErr bool
	)
	for {
		end := strings.IndexFunc(goproxy, func(r rune) bool { return r == ',' || r == '|' })
		if end < 0 {
			switch goproxy {
			case "direct", "off", "":
				return nil, fmt.Errorf("no proxy URL found")
			}
			return newClient(goproxy, hc), nil
		}
		part := goproxy[:end]
		switch part {
		case "direct", "off", "":
			goproxy = goproxy[end+1:]
			continue
		}
		first = newClient(part, hc)
		afterAnyErr = goproxy[end] == '|'
		goproxy = goproxy[end+1:]
		break
	}

	var rest []nextClient

	for goproxy != "" {
		end := strings.IndexFunc(goproxy, func(r rune) bool { return r == ',' || r == '|' })
		if end < 0 {
			switch goproxy {
			case "direct", "off", "":
				// do nothing
			default:
				rest = append(rest, nextClient{client: newClient(goproxy, hc), afterAnyErr: afterAnyErr})
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
		rest = append(rest, nextClient{client: newClient(part, hc), afterAnyErr: afterAnyErr})
		afterAnyErr = goproxy[end] == '|'
		goproxy = goproxy[end+1:]
	}

	return multi{first: first, rest: rest}, nil
}

func (m multi) loop(errptr *error, f func(Client)) {
	f(m.first)
	if *errptr == nil {
		return
	}
	for _, next := range m.rest {
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

func (m multi) Info(ctx context.Context, mod, ver string) (string, time.Time, map[string]json.RawMessage, error) {
	var (
		canonicalVer string
		tm           time.Time
		j            map[string]json.RawMessage
		err          error
	)

	m.loop(&err, func(c Client) {
		canonicalVer, tm, j, err = c.Info(ctx, mod, ver)
	})

	return canonicalVer, tm, j, err
}

func (m multi) Latest(ctx context.Context, mod string) (string, time.Time, map[string]json.RawMessage, error) {
	var (
		canonicalVer string
		tm           time.Time
		j            map[string]json.RawMessage
		err          error
	)

	m.loop(&err, func(c Client) {
		canonicalVer, tm, j, err = c.Latest(ctx, mod)
	})

	return canonicalVer, tm, j, err
}

func (m multi) List(ctx context.Context, mod string) ([]string, error) {
	var (
		versions []string
		err      error
	)

	m.loop(&err, func(c Client) {
		versions, err = c.List(ctx, mod)
	})

	return versions, err
}

func (m multi) Mod(ctx context.Context, mod, ver string) (io.ReadCloser, error) {
	var (
		rc  io.ReadCloser
		err error
	)

	m.loop(&err, func(c Client) {
		rc, err = c.Mod(ctx, mod, ver)
	})

	return rc, err
}

func (m multi) Zip(ctx context.Context, mod, ver string) (io.ReadCloser, error) {
	var (
		rc  io.ReadCloser
		err error
	)

	m.loop(&err, func(c Client) {
		rc, err = c.Zip(ctx, mod, ver)
	})

	return rc, err
}
