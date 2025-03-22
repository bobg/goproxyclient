package goproxyclient

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bobg/mid"
)

func TestClients(t *testing.T) {
	s1 := httptest.NewServer(testHandler(map[string]int{
		"github.com/bobg/mid":    http.StatusNotFound,
		"github.com/bobg/subcmd": http.StatusForbidden,
	}))
	defer s1.Close()

	t.Log("s1.URL:", s1.URL)

	ctx := context.Background()

	var (
		wantErrorsList    = []string{"v0.10.0", "v1.0.0", "v1.1.0"}
		wantErrorsVersion = "v1.1.0"
		wantErrorsTime    = time.Date(2024, 5, 15, 17, 43, 47, 0, time.UTC)
	)

	t.Run("single", func(t *testing.T) {
		cl := New(s1.URL, nil)

		t.Run("latest", func(t *testing.T) {
			ver, tm, _, err := cl.Latest(ctx, "github.com/bobg/errors")
			if err != nil {
				t.Fatal(err)
			}
			if ver != wantErrorsVersion {
				t.Errorf("got %q, want v1.1.0", wantErrorsVersion)
			}
			if !tm.Equal(wantErrorsTime) {
				t.Errorf("got %s, want %s", tm, wantErrorsTime)
			}
		})

		t.Run("list", func(t *testing.T) {
			versions, err := cl.List(ctx, "github.com/bobg/errors")
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(versions, wantErrorsList) {
				t.Errorf("got %v, want %v", versions, wantErrorsList)
			}
		})

		t.Run("info", func(t *testing.T) {
			ver, tm, _, err := cl.Info(ctx, "github.com/bobg/errors", wantErrorsVersion)
			if err != nil {
				t.Fatal(err)
			}
			if ver != wantErrorsVersion {
				t.Errorf("got %q, want v1.1.0", wantErrorsVersion)
			}
			if !tm.Equal(wantErrorsTime) {
				t.Errorf("got %s, want %s", tm, wantErrorsTime)
			}
		})

		t.Run("not_found", func(t *testing.T) {
			_, err := cl.List(ctx, "github.com/bobg/mid")
			if err == nil {
				t.Fatal("got nil, want error")
			}
			if !IsNotFound(err) {
				t.Error("IsNotFound is false, want true")
			}
		})

		t.Run("forbidden", func(t *testing.T) {
			_, err := cl.List(ctx, "github.com/bobg/subcmd/v2")
			if err == nil {
				t.Fatal("got nil, want error")
			}
			var codeErr CodeErr
			if !errors.As(err, &codeErr) {
				t.Fatalf("got %v, want a CodeErr", err)
			}
			if code := codeErr.Code(); code != http.StatusForbidden {
				t.Errorf("got %d, want %d", codeErr.Code(), http.StatusForbidden)
			}
		})
	})

	s2 := httptest.NewServer(testHandler(nil))
	defer s2.Close()

	t.Log("s2.URL:", s2.URL)

	var (
		wantMidList = []string{
			"v1.0.0",
			"v1.0.1",
			"v1.0.2",
			"v1.1.0",
			"v1.2.0",
			"v1.2.1",
			"v1.2.2",
			"v1.2.3",
			"v1.3.0",
			"v1.3.1",
			"v1.3.2",
			"v1.3.3",
			"v1.4.0",
			"v1.4.1",
			"v1.4.2",
			"v1.5.0",
			"v1.6.0",
			"v1.7.0",
			"v1.7.1",
			"v1.8.0",
			"v1.9.0",
		}
		wantMidVersion = "v1.9.0"
		wantMidTime    = time.Date(2025, 3, 22, 17, 50, 33, 0, time.UTC)

		wantSubcmdList = []string{
			"v2.0.0",
			"v2.0.1",
			"v2.1.0",
			"v2.2.0",
			"v2.2.1",
			"v2.2.2",
			"v2.3.0",
		}
		wantSubcmdVersion = "v2.3.0"
		wantSubcmdTime    = time.Date(2024, 7, 3, 15, 49, 15, 0, time.UTC)
	)

	t.Run("multi", func(t *testing.T) {
		t.Run("after_any_error", func(t *testing.T) {
			cl, err := NewMulti(fmt.Sprintf("%s|%s", s1.URL, s2.URL), nil)
			if err != nil {
				t.Fatal(err)
			}

			t.Run("mid", func(t *testing.T) {
				t.Run("latest", func(t *testing.T) {
					ver, tm, _, err := cl.Latest(ctx, "github.com/bobg/mid")
					if err != nil {
						t.Fatal(err)
					}
					if ver != wantMidVersion {
						t.Errorf("got %q, want v1.1.0", wantMidVersion)
					}
					if !tm.Equal(wantMidTime) {
						t.Errorf("got %s, want %s", tm, wantMidTime)
					}
				})
				t.Run("list", func(t *testing.T) {
					versions, err := cl.List(ctx, "github.com/bobg/mid")
					if err != nil {
						t.Fatal(err)
					}
					if !slices.Equal(versions, wantMidList) {
						t.Errorf("got %v, want %v", versions, wantMidList)
					}
				})

				t.Run("info", func(t *testing.T) {
					ver, tm, _, err := cl.Info(ctx, "github.com/bobg/mid", wantMidVersion)
					if err != nil {
						t.Fatal(err)
					}
					if ver != wantMidVersion {
						t.Errorf("got %q, want v1.1.0", wantMidVersion)
					}
					if !tm.Equal(wantMidTime) {
						t.Errorf("got %s, want %s", tm, wantMidTime)
					}
				})
			})

			t.Run("subcmd", func(t *testing.T) {
				t.Run("latest", func(t *testing.T) {
					ver, tm, _, err := cl.Latest(ctx, "github.com/bobg/subcmd/v2")
					if err != nil {
						t.Fatal(err)
					}
					if ver != wantSubcmdVersion {
						t.Errorf("got %q, want v1.1.0", wantSubcmdVersion)
					}
					if !tm.Equal(wantSubcmdTime) {
						t.Errorf("got %s, want %s", tm, wantSubcmdTime)
					}
				})
				t.Run("list", func(t *testing.T) {
					versions, err := cl.List(ctx, "github.com/bobg/subcmd/v2")
					if err != nil {
						t.Fatal(err)
					}
					if !slices.Equal(versions, wantSubcmdList) {
						t.Errorf("got %v, want %v", versions, wantSubcmdList)
					}
				})

				t.Run("info", func(t *testing.T) {
					ver, tm, _, err := cl.Info(ctx, "github.com/bobg/subcmd/v2", wantSubcmdVersion)
					if err != nil {
						t.Fatal(err)
					}
					if ver != wantSubcmdVersion {
						t.Errorf("got %q, want v1.1.0", wantSubcmdVersion)
					}
					if !tm.Equal(wantSubcmdTime) {
						t.Errorf("got %s, want %s", tm, wantSubcmdTime)
					}
				})
			})
		})

		t.Run("after_not_found", func(t *testing.T) {
			cl, err := NewMulti(fmt.Sprintf("%s,%s", s1.URL, s2.URL), nil)
			if err != nil {
				t.Fatal(err)
			}

			t.Run("mid", func(t *testing.T) {
				t.Run("latest", func(t *testing.T) {
					ver, tm, _, err := cl.Latest(ctx, "github.com/bobg/mid")
					if err != nil {
						t.Fatal(err)
					}
					if ver != wantMidVersion {
						t.Errorf("got %q, want v1.1.0", wantMidVersion)
					}
					if !tm.Equal(wantMidTime) {
						t.Errorf("got %s, want %s", tm, wantMidTime)
					}
				})
				t.Run("list", func(t *testing.T) {
					versions, err := cl.List(ctx, "github.com/bobg/mid")
					if err != nil {
						t.Fatal(err)
					}
					if !slices.Equal(versions, wantMidList) {
						t.Errorf("got %v, want %v", versions, wantMidList)
					}
				})

				t.Run("info", func(t *testing.T) {
					ver, tm, _, err := cl.Info(ctx, "github.com/bobg/mid", wantMidVersion)
					if err != nil {
						t.Fatal(err)
					}
					if ver != wantMidVersion {
						t.Errorf("got %q, want v1.1.0", wantMidVersion)
					}
					if !tm.Equal(wantMidTime) {
						t.Errorf("got %s, want %s", tm, wantMidTime)
					}
				})
			})

			t.Run("subcmd", func(t *testing.T) {
				t.Run("latest", func(t *testing.T) {
					_, _, _, err := cl.Latest(ctx, "github.com/bobg/subcmd/v2")
					if err == nil {
						t.Fatal("got nil, want error")
					}
					var codeErr CodeErr
					if !errors.As(err, &codeErr) {
						t.Fatalf("got %v, want a CodeErr", err)
					}
					if code := codeErr.Code(); code != http.StatusForbidden {
						t.Errorf("got %d, want %d", codeErr.Code(), http.StatusForbidden)
					}
				})
				t.Run("list", func(t *testing.T) {
					_, err := cl.List(ctx, "github.com/bobg/subcmd/v2")
					var codeErr CodeErr
					if !errors.As(err, &codeErr) {
						t.Fatalf("got %v, want a CodeErr", err)
					}
					if code := codeErr.Code(); code != http.StatusForbidden {
						t.Errorf("got %d, want %d", codeErr.Code(), http.StatusForbidden)
					}
				})

				t.Run("info", func(t *testing.T) {
					_, _, _, err := cl.Info(ctx, "github.com/bobg/subcmd/v2", wantSubcmdVersion)
					var codeErr CodeErr
					if !errors.As(err, &codeErr) {
						t.Fatalf("got %v, want a CodeErr", err)
					}
					if code := codeErr.Code(); code != http.StatusForbidden {
						t.Errorf("got %d, want %d", codeErr.Code(), http.StatusForbidden)
					}
				})
			})
		})
	})
}

func testHandler(shortcircuit map[string]int) http.Handler {
	return mid.Err(func(w http.ResponseWriter, req *http.Request) error {
		reqPath := strings.Trim(req.URL.Path, "/")
		for path, code := range shortcircuit {
			if strings.HasPrefix(reqPath, path) {
				return mid.CodeErr{C: code}
			}
		}

		http.ServeFileFS(w, req, testdata, filepath.Join("testdata", reqPath))
		return nil
	})
}

//go:embed testdata
var testdata embed.FS
