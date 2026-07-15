package release

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DefaultRepo is the GitHub owner/repo the updater reads releases from. The Go
// module path is unrelated; releases are published from this repository.
const DefaultRepo = "AsaphNoam/AgentDeck"

// GitHubFetcher resolves and downloads releases from GitHub. It performs network
// I/O only when explicitly invoked by `agentdeck update`; there is no background
// polling, telemetry, or auto-update (FS-10.R7, TS-06.R19).
type GitHubFetcher struct {
	Repo   string
	Client *http.Client
}

// NewGitHubFetcher returns a fetcher for repo (default DefaultRepo) with a
// bounded HTTP client.
func NewGitHubFetcher(repo string) *GitHubFetcher {
	if repo == "" {
		repo = DefaultRepo
	}
	return &GitHubFetcher{Repo: repo, Client: &http.Client{Timeout: 60 * time.Second}}
}

// Latest resolves the newest release's tag, then downloads and parses its
// published manifest.json (TS-06.R17).
func (g *GitHubFetcher) Latest(ctx context.Context) (ReleaseManifest, error) {
	var rel struct {
		TagName string `json:"tag_name"`
	}
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", g.Repo)
	if err := g.getJSON(ctx, apiURL, &rel); err != nil {
		return ReleaseManifest{}, fmt.Errorf("resolve latest release: %w", err)
	}
	if rel.TagName == "" {
		return ReleaseManifest{}, fmt.Errorf("latest release has no tag")
	}
	manifestURL := fmt.Sprintf("https://github.com/%s/releases/download/%s/manifest.json", g.Repo, rel.TagName)
	var m ReleaseManifest
	if err := g.getJSON(ctx, manifestURL, &m); err != nil {
		return ReleaseManifest{}, fmt.Errorf("fetch release manifest: %w", err)
	}
	if err := m.Validate(); err != nil {
		return ReleaseManifest{}, err
	}
	return m, nil
}

// Download fetches the archive named by the manifest into destDir.
func (g *GitHubFetcher) Download(ctx context.Context, m ReleaseManifest, destDir string) (string, error) {
	tag := "v" + strings.TrimPrefix(m.Version, "v")
	archiveURL := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", g.Repo, tag, m.Archive)
	dest := filepath.Join(destDir, m.Archive)
	if err := g.getFile(ctx, archiveURL, dest); err != nil {
		return "", fmt.Errorf("download %s: %w", m.Archive, err)
	}
	return dest, nil
}

func (g *GitHubFetcher) getJSON(ctx context.Context, url string, v any) error {
	body, err := g.get(ctx, url)
	if err != nil {
		return err
	}
	defer body.Close()
	return json.NewDecoder(io.LimitReader(body, 1<<20)).Decode(v)
}

func (g *GitHubFetcher) getFile(ctx context.Context, url, dest string) error {
	body, err := g.get(ctx, url)
	if err != nil {
		return err
	}
	defer body.Close()
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, io.LimitReader(body, maxArchiveBytes)); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func (g *GitHubFetcher) get(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/octet-stream, application/json")
	resp, err := g.Client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return resp.Body, nil
}
