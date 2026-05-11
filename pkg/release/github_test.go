package release

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return &Client{
		BaseURL: srv.URL,
		Owner:   "guneet",
		Repo:    "easyinfra",
	}, srv
}

func TestLatestReleaseSuccess(t *testing.T) {
	want := Release{
		TagName: "v0.1.0",
		Name:    "v0.1.0",
		Assets: []Asset{
			{Name: "easyinfra_Linux_x86_64.tar.gz", BrowserDownloadURL: "https://example.com/linux.tar.gz"},
		},
		PublishedAt: time.Now(),
	}
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/repos/guneet/easyinfra/releases/latest", r.URL.Path)
		require.Equal(t, "application/vnd.github+json", r.Header.Get("Accept"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(want)
	})
	got, err := client.LatestRelease(context.Background())
	require.NoError(t, err)
	require.Equal(t, "v0.1.0", got.TagName)
	require.Len(t, got.Assets, 1)
}

func TestLatestReleaseNotFound(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	_, err := client.LatestRelease(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "no releases found")
}

func TestLatestReleaseRateLimit(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-RateLimit-Reset", "1715000000")
		w.WriteHeader(http.StatusForbidden)
	})
	_, err := client.LatestRelease(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "rate limit")
	require.Contains(t, err.Error(), "1715000000")
}

func TestLatestReleaseUnexpectedStatus(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	_, err := client.LatestRelease(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "status 500")
}

func TestLatestReleaseDefaultBaseURL(t *testing.T) {
	c := &Client{Owner: "x", Repo: "y"}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err := c.LatestRelease(ctx)
	require.Error(t, err)
}

func TestFindAssetLinuxAmd64(t *testing.T) {
	release := &Release{Assets: []Asset{
		{Name: "easyinfra_Linux_x86_64.tar.gz"},
		{Name: "easyinfra_Darwin_arm64.tar.gz"},
		{Name: "easyinfra_Windows_x86_64.zip"},
	}}
	asset, err := FindAsset(release, "linux", "amd64")
	require.NoError(t, err)
	require.Equal(t, "easyinfra_Linux_x86_64.tar.gz", asset.Name)
}

func TestFindAssetLinuxArm64(t *testing.T) {
	release := &Release{Assets: []Asset{
		{Name: "easyinfra_Linux_arm64.tar.gz"},
	}}
	asset, err := FindAsset(release, "linux", "arm64")
	require.NoError(t, err)
	require.Equal(t, "easyinfra_Linux_arm64.tar.gz", asset.Name)
}

func TestFindAssetDarwinArm64(t *testing.T) {
	release := &Release{Assets: []Asset{
		{Name: "easyinfra_Linux_x86_64.tar.gz"},
		{Name: "easyinfra_Darwin_arm64.tar.gz"},
	}}
	asset, err := FindAsset(release, "darwin", "arm64")
	require.NoError(t, err)
	require.Equal(t, "easyinfra_Darwin_arm64.tar.gz", asset.Name)
}

func TestFindAssetWindowsAmd64(t *testing.T) {
	release := &Release{Assets: []Asset{
		{Name: "easyinfra_Windows_x86_64.zip"},
	}}
	asset, err := FindAsset(release, "windows", "amd64")
	require.NoError(t, err)
	require.Equal(t, "easyinfra_Windows_x86_64.zip", asset.Name)
}

func TestFindAssetMiss(t *testing.T) {
	release := &Release{Assets: []Asset{
		{Name: "easyinfra_Linux_x86_64.tar.gz"},
	}}
	_, err := FindAsset(release, "freebsd", "amd64")
	require.Error(t, err)
	require.Contains(t, err.Error(), "easyinfra_Linux_x86_64.tar.gz")
}

func TestTokenHeader(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer mytoken", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Release{TagName: "v1.0.0"})
	})
	client.Token = "mytoken"
	_, err := client.LatestRelease(context.Background())
	require.NoError(t, err)
}
