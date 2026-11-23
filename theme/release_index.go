package theme

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// ReleaseIndexPath is the public single-theme index path on a release website.
const ReleaseIndexPath = "/.hypertexting/themes.json"

// ReleaseIndex describes one theme. Archive locations come exclusively from
// Versions; clients must not construct them from a theme name or version.
type ReleaseIndex struct {
	Name     string             `json:"name"`
	Latest   string             `json:"latest"`
	Versions map[string]Release `json:"versions"`
}

// Release identifies the exact downloadable archive bytes for one version.
// URL may be absolute, root-relative, or relative to the index URL.
type Release struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

func (index ReleaseIndex) Validate() error {
	if strings.TrimSpace(index.Name) == "" || index.Latest == "" || len(index.Versions) == 0 {
		return fmt.Errorf("theme index requires name, latest, and versions")
	}
	if _, exists := index.Versions[index.Latest]; !exists {
		return fmt.Errorf("theme index latest %q is missing from versions", index.Latest)
	}
	for version, release := range index.Versions {
		if version == "" {
			return fmt.Errorf("theme index contains an empty version")
		}
		digest, err := hex.DecodeString(release.SHA256)
		if err != nil || len(digest) != 32 {
			return fmt.Errorf("theme index version %q requires a SHA-256 digest", version)
		}
		uri, err := url.Parse(release.URL)
		if err != nil || release.URL == "" || uri.Fragment != "" || uri.User != nil || (uri.Scheme != "" && uri.Scheme != "http" && uri.Scheme != "https") || (uri.IsAbs() && uri.Host == "") {
			return fmt.Errorf("theme index version %q has an invalid archive URL", version)
		}
	}
	return nil
}

// ReleaseIndexURL expands only website roots. Explicit non-root paths,
// including their escaping and query strings, are retained as supplied.
func ReleaseIndexURL(source string) (string, error) {
	uri, err := url.Parse(source)
	if err != nil || (uri.Scheme != "http" && uri.Scheme != "https") || uri.Host == "" || uri.User != nil || uri.Fragment != "" {
		return "", fmt.Errorf("invalid theme index source URL %q", source)
	}
	if uri.EscapedPath() == "" || uri.EscapedPath() == "/" {
		uri.Path = ReleaseIndexPath
		uri.RawPath = ""
		return uri.String(), nil
	}
	return source, nil
}

// FetchReleaseIndex fetches a website-root shorthand or an explicit index URL.
// Relative archive URLs resolve against the final index URL after redirects.
// The client is injectable; nil uses http.DefaultClient.
func FetchReleaseIndex(ctx context.Context, client *http.Client, source string) (ReleaseIndex, error) {
	indexURL, err := ReleaseIndexURL(source)
	if err != nil {
		return ReleaseIndex{}, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, indexURL, nil)
	if err != nil {
		return ReleaseIndex{}, err
	}
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(request)
	if err != nil {
		return ReleaseIndex{}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return ReleaseIndex{}, fmt.Errorf("fetch theme index: %s", response.Status)
	}
	var index ReleaseIndex
	decoder := json.NewDecoder(response.Body)
	if err := decoder.Decode(&index); err != nil {
		return ReleaseIndex{}, fmt.Errorf("decode theme index: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return ReleaseIndex{}, fmt.Errorf("theme index must contain one JSON object")
	}
	if err := index.Validate(); err != nil {
		return ReleaseIndex{}, err
	}
	for version, release := range index.Versions {
		ref, _ := url.Parse(release.URL) // validated above
		release.URL = response.Request.URL.ResolveReference(ref).String()
		index.Versions[version] = release
	}
	return index, nil
}

// ResolveRelease fetches an installed theme's config.url and selects a version
// (empty means latest). It does not download, install, or update any theme.
// Loading a theme stays offline; consumers opt in to this shared resolver.
func (theme *Theme) ResolveRelease(ctx context.Context, client *http.Client, version string) (Release, error) {
	index, err := FetchReleaseIndex(ctx, client, theme.Config.URL)
	if err != nil {
		return Release{}, err
	}
	if index.Name != theme.Name {
		return Release{}, fmt.Errorf("theme index name %q does not match installed theme %q", index.Name, theme.Name)
	}
	if version == "" {
		version = index.Latest
	}
	release, ok := index.Versions[version]
	if !ok {
		return Release{}, fmt.Errorf("theme version %q is missing from index", version)
	}
	return release, nil
}
