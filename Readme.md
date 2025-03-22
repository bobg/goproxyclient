# Goproxyclient

[![Go Reference](https://pkg.go.dev/badge/github.com/bobg/goproxyclient.svg)](https://pkg.go.dev/github.com/bobg/goproxyclient)
[![Go Report Card](https://goreportcard.com/badge/github.com/bobg/goproxyclient)](https://goreportcard.com/report/github.com/bobg/goproxyclient)
[![Tests](https://github.com/bobg/goproxyclient/actions/workflows/go.yml/badge.svg)](https://github.com/bobg/goproxyclient/actions/workflows/go.yml)
[![Coverage Status](https://coveralls.io/repos/github/bobg/goproxyclient/badge.svg?branch=master)](https://coveralls.io/github/bobg/goproxyclient?branch=master)

This is goproxyclient,
a library and command-line tool
for communicating with a [Go module proxy](https://go.dev/ref/mod#module-proxy),
such as the public one operated by Google at [proxy.golang.org](https://proxy.golang.org).

## Installation

For the command-line tool:

```sh
go install github.com/bobg/goproxyclient/cmd/goproxyclient@latest
```

For the library:

```sh
go get github.com/bobg/goproxyclient@latest
```

## Usage

For library usage please see
[the package doc](https://pkg.go.dev/github.com/bobg/goproxyclient).

Command-line usage:

```sh
goproxyclient [-proxy URL] COMMAND ARG ARG...
```

where COMMAND is one of `info`, `latest`, `list`, `mod`, and `zip`.
If `-proxy` is given,
it is the base URL of the Go module proxy server to query.
The default is the first element of the `GOPROXY` environment variable,
or `https://proxy.golang.org` if that’s not set.

The `info` command produces JSON-encoded metadata about each argument.
Each argument must be in the form MODPATH@VERSION.

The `latest` command produces JSON-encoded metadata about the latest version of each argument.
Each argument must be a bare module path.

The `list` command produces a sorted list of available versions for each argument.
Each argument must be a bare module path.

The `mod` command produces the `go.mod` file for its argument,
which must be in the form MODPATH@VERSION.

The `zip` command produces a zip file with the module contents for its argument,
which must be in the form MODPATH@VERSION.
