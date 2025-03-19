package goproxyclient

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/bobg/errors"
)

// Client is the type of a client for talking to a Go module proxy.
type Client struct {
	baseURL string
	client  *http.Client
}

// New creates a new [Client] for the Go module proxy at the given URL.
// If client is non-nil, it will use that HTTP client,
// otherwise it will use a default client
// (a distinct one from [http.DefaultClient]).
func New(url string, client *http.Client) *Client {
	url = strings.TrimRight(url, "/")
	if client == nil {
		client = &http.Client{}
	}
	return &Client{baseURL: url, client: client}
}

// List lists the available versions of a Go module.
func (c Client) List(ctx context.Context, modpath string) ([]string, error) {
	q := fmt.Sprintf("%s/%s/@v/list", c.baseURL, modpath)

	req, err := http.NewRequestWithContext(ctx, "GET", q, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "creating GET %s request", q)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "in GET %s", q)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", q, resp.Status)
	}

	var (
		sc       = bufio.NewScanner(resp.Body)
		versions []string
	)
	for sc.Scan() {
		versions = append(versions, sc.Text())
	}
	return versions, errors.Wrapf(sc.Err(), "scanning response from GET %s", q)
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
func (c Client) Info(ctx context.Context, modpath, version string) (string, time.Time, map[string]json.RawMessage, error) {
	q := fmt.Sprintf("%s/%s/@v/%s.info", c.baseURL, modpath, version)
	return c.handleInfoRequest(ctx, q)
}

// Mod gets the go.mod file for a specific version of a Go module.
func (c Client) Mod(ctx context.Context, modpath, version string) (io.ReadCloser, error) {
	return c.getContent(ctx, modpath, version, "mod")
}

// Zip gets the contents of a specific version of a Go module as a zip file.
func (c Client) Zip(ctx context.Context, modpath, version string) (io.ReadCloser, error) {
	return c.getContent(ctx, modpath, version, "zip")
}

func (c Client) getContent(ctx context.Context, modpath, version, suffix string) (io.ReadCloser, error) {
	q := fmt.Sprintf("%s/%s/@v/%s.%s", c.baseURL, modpath, version, suffix)

	req, err := http.NewRequestWithContext(ctx, "GET", q, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "creating GET %s request", q)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "in GET %s", q)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("GET %s: %s", q, resp.Status)
	}

	return resp.Body, nil
}

// Latest gets info about the latest version of a Go module.
// Its return values are the same as for [Info].
func (c Client) Latest(ctx context.Context, modpath string) (string, time.Time, map[string]json.RawMessage, error) {
	q := fmt.Sprintf("%s/%s/@latest", c.baseURL, modpath)
	return c.handleInfoRequest(ctx, q)
}

func (c Client) handleInfoRequest(ctx context.Context, q string) (string, time.Time, map[string]json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", q, nil)
	if err != nil {
		return "", time.Time{}, nil, errors.Wrapf(err, "creating GET %s request", q)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return "", time.Time{}, nil, errors.Wrapf(err, "in GET %s", q)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", time.Time{}, nil, fmt.Errorf("GET %s: %s", q, resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", time.Time{}, nil, errors.Wrapf(err, "reading response body from GET %s", q)
	}

	var info struct {
		Version string
		Time    time.Time
	}

	if err := json.Unmarshal(body, &info); err != nil {
		return "", time.Time{}, nil, errors.Wrapf(err, "unmarshaling response body from GET %s", q)
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(body, &m); err != nil {
		return "", time.Time{}, nil, errors.Wrapf(err, "unmarshaling response body from GET %s", q)
	}

	return info.Version, info.Time, m, nil
}
