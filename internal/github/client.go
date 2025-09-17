package github

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/go-github/v57/github"
	"golang.org/x/oauth2"
)

// Client wraps the GitHub API client
type Client struct {
	client         *github.Client
	ctx            context.Context
	repositoryName string
	dryRun         bool
}

// NewClient creates a new GitHub client
func NewClient(token, repositoryName string, dryRun bool) *Client {
	c := &Client{
		repositoryName: repositoryName,
		ctx:            context.Background(),
		dryRun:         dryRun,
	}

	// Only initialize GitHub API connection for PR operations (not in dry-run)
	if !dryRun {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)
		tc := oauth2.NewClient(c.ctx, ts)
		c.client = github.NewClient(tc)
	}

	return c
}

// GetExistingPullRequests gets all existing pull request head branch names and their states
func (c *Client) GetExistingPullRequests() (map[string]string, error) {
	if c.dryRun {
		fmt.Println("[DRY RUN] Would check existing pull requests with GitHub API")
		// Return empty map for dry-run to simulate no existing PRs
		return make(map[string]string), nil
	}

	// Parse repository name
	parts := strings.Split(c.repositoryName, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repository name format: %s", c.repositoryName)
	}
	owner, repo := parts[0], parts[1]

	// Get all pull requests
	opts := &github.PullRequestListOptions{
		State: "all",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	allPRs := make(map[string]string)

	for {
		prs, resp, err := c.client.PullRequests.List(c.ctx, owner, repo, opts)
		if err != nil {
			return nil, fmt.Errorf("error getting pull requests: %w", err)
		}

		for _, pr := range prs {
			if pr.Head != nil && pr.Head.Ref != nil && pr.State != nil {
				allPRs[*pr.Head.Ref] = *pr.State
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allPRs, nil
}

// CreatePullRequest creates a pull request for the assignment branch
func (c *Client) CreatePullRequest(title, body, head, base string) (string, error) {
	if c.dryRun {
		fmt.Printf("[DRY RUN] Would create pull request:\n")
		fmt.Printf("  Title: %s\n", title)
		fmt.Printf("  Head: %s\n", head)
		fmt.Printf("  Base: %s\n", base)
		bodyPreview := body
		if len(body) > 100 {
			bodyPreview = body[:100] + "..."
		}
		fmt.Printf("  Body: %s\n", bodyPreview)

		// Simulate PR number (this would need to be passed in for proper simulation)
		fmt.Printf("[DRY RUN] Simulated pull request #1\n")
		return "#1", nil
	}

	// Parse repository name
	parts := strings.Split(c.repositoryName, "/")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid repository name format: %s", c.repositoryName)
	}
	owner, repo := parts[0], parts[1]

	// Create the pull request via GitHub API
	newPR := &github.NewPullRequest{
		Title: &title,
		Body:  &body,
		Head:  &head,
		Base:  &base,
	}

	pr, _, err := c.client.PullRequests.Create(c.ctx, owner, repo, newPR)
	if err != nil {
		return "", fmt.Errorf("error creating pull request: %w", err)
	}

	prNumber := fmt.Sprintf("#%d", *pr.Number)
	fmt.Printf("✅ Created pull request %s: %s\n", prNumber, title)
	return prNumber, nil
}

// MergePullRequest merges a pull request automatically using the merge commit strategy
func (c *Client) MergePullRequest(prNumber, title string) error {
	if c.dryRun {
		fmt.Printf("[DRY RUN] Would merge pull request %s\n", prNumber)
		return nil
	}

	// Convert PR number string to integer (remove # prefix if present)
	prNum, err := strconv.Atoi(strings.TrimPrefix(prNumber, "#"))
	if err != nil {
		return fmt.Errorf("invalid PR number format '%s': %w", prNumber, err)
	}

	// Parse repository name
	parts := strings.Split(c.repositoryName, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid repository name format: %s", c.repositoryName)
	}
	owner, repo := parts[0], parts[1]

	// Merge the pull request using merge commit strategy
	commitMessage := fmt.Sprintf("Merge pull request %s: %s", prNumber, title)
	mergeOptions := &github.PullRequestOptions{
		CommitTitle: commitMessage,
		MergeMethod: "merge", // Use merge commit strategy
	}

	result, _, err := c.client.PullRequests.Merge(c.ctx, owner, repo, prNum, "", mergeOptions)
	if err != nil {
		return fmt.Errorf("error merging pull request %s: %w", prNumber, err)
	}

	if result.Merged != nil && *result.Merged {
		fmt.Printf("✅ Merged pull request %s\n", prNumber)
	} else {
		return fmt.Errorf("failed to merge pull request %s: merge was not successful", prNumber)
	}

	return nil
}

// ReopenPullRequest reopens a closed pull request
func (c *Client) ReopenPullRequest(prNumber, title string) error {
	if c.dryRun {
		fmt.Printf("[DRY RUN] Would reopen pull request %s\n", prNumber)
		return nil
	}

	// Convert PR number string to integer (remove # prefix if present)
	prNum, err := strconv.Atoi(strings.TrimPrefix(prNumber, "#"))
	if err != nil {
		return fmt.Errorf("invalid PR number format '%s': %w", prNumber, err)
	}

	// Parse repository name
	parts := strings.Split(c.repositoryName, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid repository name format: %s", c.repositoryName)
	}
	owner, repo := parts[0], parts[1]

	// Reopen the pull request by setting state to "open"
	state := "open"
	prUpdate := &github.PullRequest{
		State: &state,
	}

	_, _, err = c.client.PullRequests.Edit(c.ctx, owner, repo, prNum, prUpdate)
	if err != nil {
		return fmt.Errorf("error reopening pull request %s: %w", prNumber, err)
	}

	fmt.Printf("✅ Reopened pull request %s\n", prNumber)
	return nil
}
