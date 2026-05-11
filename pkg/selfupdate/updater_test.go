package selfupdate

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUpdaterFields(t *testing.T) {
	u := &Updater{
		Owner:          "guneet",
		Repo:           "easyinfra",
		CurrentVersion: "v0.1.0",
	}
	require.Equal(t, "guneet", u.Owner)
	require.Equal(t, "easyinfra", u.Repo)
	require.Equal(t, "v0.1.0", u.CurrentVersion)
}

func TestCheckResultFields(t *testing.T) {
	r := &CheckResult{
		LatestVersion: "v0.2.0",
		HasUpdate:     true,
		DownloadURL:   "https://example.com/download",
	}
	require.True(t, r.HasUpdate)
	require.Equal(t, "v0.2.0", r.LatestVersion)
	require.Equal(t, "https://example.com/download", r.DownloadURL)
}

func TestGitHubRepositoryGetSlug(t *testing.T) {
	repo := &GitHubRepository{
		Owner: "guneet",
		Repo:  "easyinfra",
	}
	owner, repoName, err := repo.GetSlug()
	require.NoError(t, err)
	require.Equal(t, "guneet", owner)
	require.Equal(t, "easyinfra", repoName)
}

func TestGitHubRepositoryGet(t *testing.T) {
	repo := &GitHubRepository{
		Owner: "guneet",
		Repo:  "easyinfra",
	}
	result, err := repo.Get()
	require.NoError(t, err)
	require.Nil(t, result)
}

func TestUpdaterCreation(t *testing.T) {
	tests := []struct {
		name    string
		owner   string
		repo    string
		version string
	}{
		{
			name:    "valid updater",
			owner:   "guneet",
			repo:    "easyinfra",
			version: "v1.0.0",
		},
		{
			name:    "different owner",
			owner:   "other-user",
			repo:    "other-repo",
			version: "v2.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &Updater{
				Owner:          tt.owner,
				Repo:           tt.repo,
				CurrentVersion: tt.version,
			}
			require.Equal(t, tt.owner, u.Owner)
			require.Equal(t, tt.repo, u.Repo)
			require.Equal(t, tt.version, u.CurrentVersion)
		})
	}
}

func TestCheckResultCreation(t *testing.T) {
	tests := []struct {
		name          string
		latestVersion string
		hasUpdate     bool
		downloadURL   string
	}{
		{
			name:          "update available",
			latestVersion: "v2.0.0",
			hasUpdate:     true,
			downloadURL:   "https://github.com/guneet/easyinfra/releases/download/v2.0.0/easyinfra",
		},
		{
			name:          "no update available",
			latestVersion: "v1.0.0",
			hasUpdate:     false,
			downloadURL:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &CheckResult{
				LatestVersion: tt.latestVersion,
				HasUpdate:     tt.hasUpdate,
				DownloadURL:   tt.downloadURL,
			}
			require.Equal(t, tt.latestVersion, r.LatestVersion)
			require.Equal(t, tt.hasUpdate, r.HasUpdate)
			require.Equal(t, tt.downloadURL, r.DownloadURL)
		})
	}
}

func TestCheckWithInvalidGitHubConfig(t *testing.T) {
	u := &Updater{
		Owner:          "guneet",
		Repo:           "easyinfra",
		CurrentVersion: "v0.1.0",
	}

	ctx := context.Background()
	result, err := u.Check(ctx)

	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestUpdateWithInvalidGitHubConfig(t *testing.T) {
	u := &Updater{
		Owner:          "guneet",
		Repo:           "easyinfra",
		CurrentVersion: "v0.1.0",
	}

	ctx := context.Background()
	version, err := u.Update(ctx)

	require.NoError(t, err)
	require.NotEmpty(t, version)
}

func TestGitHubRepositoryMultipleInstances(t *testing.T) {
	repo1 := &GitHubRepository{
		Owner: "owner1",
		Repo:  "repo1",
	}
	repo2 := &GitHubRepository{
		Owner: "owner2",
		Repo:  "repo2",
	}

	owner1, repo1Name, err1 := repo1.GetSlug()
	owner2, repo2Name, err2 := repo2.GetSlug()

	require.NoError(t, err1)
	require.NoError(t, err2)
	require.Equal(t, "owner1", owner1)
	require.Equal(t, "repo1", repo1Name)
	require.Equal(t, "owner2", owner2)
	require.Equal(t, "repo2", repo2Name)
}

func TestUpdaterWithEmptyVersion(t *testing.T) {
	u := &Updater{
		Owner:          "guneet",
		Repo:           "easyinfra",
		CurrentVersion: "",
	}
	require.Equal(t, "", u.CurrentVersion)
}

func TestCheckResultWithEmptyURL(t *testing.T) {
	r := &CheckResult{
		LatestVersion: "v1.0.0",
		HasUpdate:     false,
		DownloadURL:   "",
	}
	require.False(t, r.HasUpdate)
	require.Equal(t, "", r.DownloadURL)
}
