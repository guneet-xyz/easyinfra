package selfupdate

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeDetector struct {
	info        *releaseInfo
	found       bool
	detectErr   error
	updateErr   error
	updateCalls int
}

func (f *fakeDetector) DetectLatest(_ context.Context, _, _ string) (*releaseInfo, bool, error) {
	return f.info, f.found, f.detectErr
}

func (f *fakeDetector) UpdateTo(_ context.Context, _ *releaseInfo, _ string) error {
	f.updateCalls++
	return f.updateErr
}

func newRelease(version, url string, greater bool) *releaseInfo {
	return &releaseInfo{
		Version:     version,
		AssetURL:    url,
		GreaterThan: func(string) bool { return greater },
	}
}

func TestCheck_HasUpdate(t *testing.T) {
	u := &Updater{
		Owner:          "guneet",
		Repo:           "easyinfra",
		CurrentVersion: "v1.0.0",
		detector: &fakeDetector{
			info:  newRelease("v2.0.0", "https://example.com/asset", true),
			found: true,
		},
	}

	res, err := u.Check(context.Background())
	require.NoError(t, err)
	require.NotNil(t, res)
	require.True(t, res.HasUpdate)
	require.Equal(t, "v2.0.0", res.LatestVersion)
	require.Equal(t, "https://example.com/asset", res.DownloadURL)
}

func TestCheck_NoUpdate_SameVersion(t *testing.T) {
	u := &Updater{
		Owner:          "guneet",
		Repo:           "easyinfra",
		CurrentVersion: "v1.0.0",
		detector: &fakeDetector{
			info:  newRelease("v1.0.0", "https://example.com/asset", false),
			found: true,
		},
	}

	res, err := u.Check(context.Background())
	require.NoError(t, err)
	require.False(t, res.HasUpdate)
	require.Equal(t, "v1.0.0", res.LatestVersion)
}

func TestCheck_NotFound(t *testing.T) {
	u := &Updater{
		Owner:          "guneet",
		Repo:           "easyinfra",
		CurrentVersion: "v1.0.0",
		detector:       &fakeDetector{found: false},
	}

	res, err := u.Check(context.Background())
	require.NoError(t, err)
	require.False(t, res.HasUpdate)
	require.Equal(t, "v1.0.0", res.LatestVersion)
}

func TestCheck_DetectorError(t *testing.T) {
	want := errors.New("network down")
	u := &Updater{
		Owner:          "guneet",
		Repo:           "easyinfra",
		CurrentVersion: "v1.0.0",
		detector:       &fakeDetector{detectErr: want},
	}

	res, err := u.Check(context.Background())
	require.Error(t, err)
	require.Nil(t, res)
	require.ErrorIs(t, err, want)
}

func TestUpdate_Applied(t *testing.T) {
	fd := &fakeDetector{
		info:  newRelease("v2.0.0", "https://example.com/asset", true),
		found: true,
	}
	u := &Updater{
		Owner:          "guneet",
		Repo:           "easyinfra",
		CurrentVersion: "v1.0.0",
		detector:       fd,
	}

	version, err := u.Update(context.Background())
	require.NoError(t, err)
	require.Equal(t, "v2.0.0", version)
	require.Equal(t, 1, fd.updateCalls)
}

func TestUpdate_NotFound_ReturnsCurrent(t *testing.T) {
	fd := &fakeDetector{found: false}
	u := &Updater{
		Owner:          "guneet",
		Repo:           "easyinfra",
		CurrentVersion: "v1.0.0",
		detector:       fd,
	}

	version, err := u.Update(context.Background())
	require.NoError(t, err)
	require.Equal(t, "v1.0.0", version)
	require.Equal(t, 0, fd.updateCalls)
}

func TestUpdate_NotGreater_ReturnsCurrent(t *testing.T) {
	fd := &fakeDetector{
		info:  newRelease("v1.0.0", "", false),
		found: true,
	}
	u := &Updater{
		Owner:          "guneet",
		Repo:           "easyinfra",
		CurrentVersion: "v1.0.0",
		detector:       fd,
	}

	version, err := u.Update(context.Background())
	require.NoError(t, err)
	require.Equal(t, "v1.0.0", version)
	require.Equal(t, 0, fd.updateCalls)
}

func TestUpdate_DetectorError(t *testing.T) {
	want := errors.New("api error")
	u := &Updater{
		Owner:          "guneet",
		Repo:           "easyinfra",
		CurrentVersion: "v1.0.0",
		detector:       &fakeDetector{detectErr: want},
	}

	version, err := u.Update(context.Background())
	require.Error(t, err)
	require.Empty(t, version)
	require.ErrorIs(t, err, want)
}

func TestUpdate_UpdateError(t *testing.T) {
	want := errors.New("disk full")
	fd := &fakeDetector{
		info:      newRelease("v2.0.0", "https://example.com/asset", true),
		found:     true,
		updateErr: want,
	}
	u := &Updater{
		Owner:          "guneet",
		Repo:           "easyinfra",
		CurrentVersion: "v1.0.0",
		detector:       fd,
	}

	version, err := u.Update(context.Background())
	require.Error(t, err)
	require.Empty(t, version)
	require.ErrorIs(t, err, want)
}

func TestGetDetector_Injected(t *testing.T) {
	fd := &fakeDetector{}
	u := &Updater{detector: fd}
	d, err := u.getDetector()
	require.NoError(t, err)
	require.Equal(t, fd, d)
}

func TestGetDetector_Real(t *testing.T) {
	u := &Updater{Owner: "x", Repo: "y", CurrentVersion: "v0.0.0"}
	d, err := u.getDetector()
	require.NoError(t, err)
	require.NotNil(t, d)
}

func TestRealDetector_DetectLatest_NetworkError(t *testing.T) {
	d, err := newRealDetector()
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	info, found, err := d.DetectLatest(ctx, "guneet", "easyinfra")
	require.Error(t, err)
	require.False(t, found)
	require.Nil(t, info)
}
