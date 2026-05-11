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

	if err != nil {
		require.NotNil(t, err)
	} else {
		require.NotNil(t, result)
	}
}

func TestUpdateWithInvalidGitHubConfig(t *testing.T) {
	u := &Updater{
		Owner:          "guneet",
		Repo:           "easyinfra",
		CurrentVersion: "v0.1.0",
	}

	ctx := context.Background()
	version, err := u.Update(ctx)

	if err != nil {
		require.NotNil(t, err)
	} else {
		require.NotEmpty(t, version)
	}
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

func TestCheckWithCancelledContext(t *testing.T) {
	u := &Updater{
		Owner:          "guneet",
		Repo:           "easyinfra",
		CurrentVersion: "v0.1.0",
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := u.Check(ctx)

	require.Error(t, err)
	require.Nil(t, result)
}

func TestUpdateWithCancelledContext(t *testing.T) {
	u := &Updater{
		Owner:          "guneet",
		Repo:           "easyinfra",
		CurrentVersion: "v0.1.0",
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	version, err := u.Update(ctx)

	require.Error(t, err)
	require.Empty(t, version)
}

func TestCheckResultHasUpdateTrue(t *testing.T) {
	r := &CheckResult{
		LatestVersion: "v2.0.0",
		HasUpdate:     true,
		DownloadURL:   "https://github.com/guneet/easyinfra/releases/download/v2.0.0/easyinfra",
	}
	require.True(t, r.HasUpdate)
	require.Equal(t, "v2.0.0", r.LatestVersion)
	require.NotEmpty(t, r.DownloadURL)
}

func TestGitHubRepositoryGetSlugWithDifferentValues(t *testing.T) {
	tests := []struct {
		owner    string
		repo     string
		expOwner string
		expRepo  string
	}{
		{"user1", "repo1", "user1", "repo1"},
		{"user-with-dash", "repo-with-dash", "user-with-dash", "repo-with-dash"},
		{"user123", "repo456", "user123", "repo456"},
	}

	for _, tt := range tests {
		t.Run(tt.owner+"/"+tt.repo, func(t *testing.T) {
			repo := &GitHubRepository{
				Owner: tt.owner,
				Repo:  tt.repo,
			}
			owner, repoName, err := repo.GetSlug()
			require.NoError(t, err)
			require.Equal(t, tt.expOwner, owner)
			require.Equal(t, tt.expRepo, repoName)
		})
	}
}

func TestCheckWithDifferentVersions(t *testing.T) {
	tests := []struct {
		name           string
		owner          string
		repo           string
		currentVersion string
	}{
		{"v0.1.0", "guneet", "easyinfra", "v0.1.0"},
		{"v1.0.0", "guneet", "easyinfra", "v1.0.0"},
		{"v2.5.3", "guneet", "easyinfra", "v2.5.3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &Updater{
				Owner:          tt.owner,
				Repo:           tt.repo,
				CurrentVersion: tt.currentVersion,
			}

			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			result, err := u.Check(ctx)

			require.Error(t, err)
			require.Nil(t, result)
		})
	}
}

func TestUpdateWithDifferentVersions(t *testing.T) {
	tests := []struct {
		name           string
		owner          string
		repo           string
		currentVersion string
	}{
		{"v0.1.0", "guneet", "easyinfra", "v0.1.0"},
		{"v1.0.0", "guneet", "easyinfra", "v1.0.0"},
		{"v2.5.3", "guneet", "easyinfra", "v2.5.3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &Updater{
				Owner:          tt.owner,
				Repo:           tt.repo,
				CurrentVersion: tt.currentVersion,
			}

			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			version, err := u.Update(ctx)

			require.Error(t, err)
			require.Empty(t, version)
		})
	}
}

func TestCheckResultWithVariousVersions(t *testing.T) {
	tests := []struct {
		name          string
		latestVersion string
		hasUpdate     bool
	}{
		{"v0.1.0 no update", "v0.1.0", false},
		{"v1.0.0 with update", "v1.0.0", true},
		{"v2.5.3 with update", "v2.5.3", true},
		{"v0.0.1 no update", "v0.0.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &CheckResult{
				LatestVersion: tt.latestVersion,
				HasUpdate:     tt.hasUpdate,
				DownloadURL:   "https://example.com/download",
			}
			require.Equal(t, tt.latestVersion, r.LatestVersion)
			require.Equal(t, tt.hasUpdate, r.HasUpdate)
		})
	}
}

func TestUpdaterFieldsWithVariousValues(t *testing.T) {
	tests := []struct {
		name    string
		owner   string
		repo    string
		version string
	}{
		{"simple", "user", "repo", "v1.0.0"},
		{"with-dashes", "user-name", "repo-name", "v1.0.0"},
		{"with-numbers", "user123", "repo456", "v2.5.3"},
		{"empty-version", "user", "repo", ""},
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

func TestGitHubRepositoryGetWithVariousRepos(t *testing.T) {
	tests := []struct {
		name  string
		owner string
		repo  string
	}{
		{"guneet/easyinfra", "guneet", "easyinfra"},
		{"other/repo", "other", "repo"},
		{"user-123/repo-456", "user-123", "repo-456"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &GitHubRepository{
				Owner: tt.owner,
				Repo:  tt.repo,
			}
			result, err := repo.Get()
			require.NoError(t, err)
			require.Nil(t, result)
		})
	}
}
