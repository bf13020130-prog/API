package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type updateCacheStub struct{}

func (updateCacheStub) GetUpdateInfo(context.Context) (string, error) {
	return "", errors.New("cache miss")
}

func (updateCacheStub) SetUpdateInfo(context.Context, string, time.Duration) error {
	return nil
}

type githubReleaseClientStub struct {
	fetchLatestCalls int
}

func (c *githubReleaseClientStub) FetchLatestRelease(context.Context, string) (*GitHubRelease, error) {
	c.fetchLatestCalls++
	return &GitHubRelease{
		TagName: "v0.1.133",
		Name:    "v0.1.133",
		HTMLURL: "https://github.com/Wei-Shaw/sub2api/releases/tag/v0.1.133",
	}, nil
}

func (c *githubReleaseClientStub) DownloadFile(context.Context, string, string, int64) error {
	return errors.New("download should not be called")
}

func (c *githubReleaseClientStub) FetchChecksumFile(context.Context, string) ([]byte, error) {
	return nil, errors.New("checksum should not be called")
}

func TestUpdateServicePerformUpdateRejectsSourceBuilds(t *testing.T) {
	client := &githubReleaseClientStub{}
	svc := NewUpdateService(updateCacheStub{}, client, "0.1.132", "source")

	err := svc.PerformUpdate(context.Background())

	require.Error(t, err)
	require.Contains(t, err.Error(), "official binary update is disabled")
	require.Equal(t, 0, client.fetchLatestCalls)
}

func TestUpdateServiceRollbackRejectsSourceBuilds(t *testing.T) {
	client := &githubReleaseClientStub{}
	svc := NewUpdateService(updateCacheStub{}, client, "0.1.132", "source")

	err := svc.Rollback()

	require.Error(t, err)
	require.Contains(t, err.Error(), "official binary rollback is disabled")
	require.Equal(t, 0, client.fetchLatestCalls)
}
