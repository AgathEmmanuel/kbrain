/*
Copyright 2026 kbrain authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package gitops

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// PROptions holds the parameters for creating a pull/merge request.
type PROptions struct {
	Platform     string // "github", "gitlab", "bitbucket"
	RepoURL      string
	SourceBranch string
	TargetBranch string
	Title        string
	Description  string
	Token        string
	AutoMerge    bool
}

// PRResult holds the result of a PR creation.
type PRResult struct {
	URL    string
	Number int
}

// Client creates pull/merge requests on git hosting platforms.
type Client struct {
	HTTPClient *http.Client
}

// NewClient creates a new gitops client.
func NewClient() *Client {
	return &Client{
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// CreatePullRequest creates a PR/MR on the specified platform.
func (c *Client) CreatePullRequest(ctx context.Context, opts PROptions) (*PRResult, error) {
	switch opts.Platform {
	case "github":
		return c.createGitHubPR(ctx, opts)
	case "gitlab":
		return c.createGitLabMR(ctx, opts)
	default:
		return nil, fmt.Errorf("unsupported platform: %s", opts.Platform)
	}
}

// parseGitHubRepo extracts owner/repo from a GitHub URL.
func parseGitHubRepo(repoURL string) (owner, repo string, err error) {
	// Handle both HTTPS and SSH URLs
	url := repoURL
	url = strings.TrimSuffix(url, ".git")
	url = strings.TrimPrefix(url, "https://github.com/")
	url = strings.TrimPrefix(url, "git@github.com:")

	parts := strings.SplitN(url, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("cannot parse GitHub repo from URL: %s", repoURL)
	}
	return parts[0], parts[1], nil
}

func (c *Client) createGitHubPR(ctx context.Context, opts PROptions) (*PRResult, error) {
	owner, repo, err := parseGitHubRepo(opts.RepoURL)
	if err != nil {
		return nil, err
	}

	payload := map[string]interface{}{
		"title": opts.Title,
		"body":  opts.Description,
		"head":  opts.SourceBranch,
		"base":  opts.TargetBranch,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshaling PR payload: %w", err)
	}

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls", owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+opts.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("creating GitHub PR: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		HTMLURL string `json:"html_url"`
		Number  int    `json:"number"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parsing GitHub response: %w", err)
	}

	return &PRResult{URL: result.HTMLURL, Number: result.Number}, nil
}

// parseGitLabProject extracts the project path from a GitLab URL.
func parseGitLabProject(repoURL string) (host, projectPath string, err error) {
	url := repoURL
	url = strings.TrimSuffix(url, ".git")

	if strings.HasPrefix(url, "https://") {
		url = strings.TrimPrefix(url, "https://")
		idx := strings.Index(url, "/")
		if idx < 0 {
			return "", "", fmt.Errorf("cannot parse GitLab project from URL: %s", repoURL)
		}
		host = url[:idx]
		projectPath = url[idx+1:]
		return host, projectPath, nil
	}

	// SSH format: git@gitlab.com:group/project
	url = strings.TrimPrefix(url, "git@")
	parts := strings.SplitN(url, ":", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("cannot parse GitLab project from URL: %s", repoURL)
	}
	return parts[0], parts[1], nil
}

func (c *Client) createGitLabMR(ctx context.Context, opts PROptions) (*PRResult, error) {
	host, projectPath, err := parseGitLabProject(opts.RepoURL)
	if err != nil {
		return nil, err
	}

	// URL-encode the project path
	encodedPath := strings.ReplaceAll(projectPath, "/", "%2F")

	payload := map[string]interface{}{
		"title":         opts.Title,
		"description":   opts.Description,
		"source_branch": opts.SourceBranch,
		"target_branch": opts.TargetBranch,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshaling MR payload: %w", err)
	}

	apiURL := fmt.Sprintf("https://%s/api/v4/projects/%s/merge_requests", host, encodedPath)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("PRIVATE-TOKEN", opts.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("creating GitLab MR: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("GitLab API returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		WebURL string `json:"web_url"`
		IID    int    `json:"iid"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parsing GitLab response: %w", err)
	}

	return &PRResult{URL: result.WebURL, Number: result.IID}, nil
}
