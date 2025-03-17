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

type Client struct {
	baseURL string
	client  *http.Client
}

func New(url string, client *http.Client) *Client {
	url = strings.TrimRight(url, "/")
	if client == nil {
		client = &http.Client{}
	}
	return &Client{baseURL: url, client: client}
}

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

func (c Client) Info(ctx context.Context, modpath, version string) (string, time.Time, map[string]any, error) {
	q := fmt.Sprintf("%s/%s/@v/%s.info", c.baseURL, modpath, version)
	return c.handleInfoRequest(ctx, q)
}

func (c Client) Mod(ctx context.Context, modpath, version string) (io.ReadCloser, error) {
	return c.getContent(ctx, modpath, version, "mod")
}

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

func (c Client) Latest(ctx context.Context, modpath string) (string, time.Time, map[string]any, error) {
	q := fmt.Sprintf("%s/%s/@latest", c.baseURL, modpath)
	return c.handleInfoRequest(ctx, q)
}

func (c Client) handleInfoRequest(ctx context.Context, q string) (string, time.Time, map[string]any, error) {
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

	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return "", time.Time{}, nil, errors.Wrapf(err, "unmarshaling response body from GET %s", q)
	}

	return info.Version, info.Time, m, nil
}
