package cmd

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/go-github/github"
	"github.com/spf13/cobra"
	"golang.org/x/oauth2"
)

var tags = []string{"chore", "docs", "feat", "fix", "refactor", "style", "test"}

// labelCmd represents the label command
var labelCmd = &cobra.Command{
	Use:   "label",
	Short: "Add labels to github PRs",
	Long: `Label labels Github PRs by matching the PR title to a predefined list of tags.
The tag is associated with a PR label, which is then applied to the PR.`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		client := initGithubClient(ctx)

		labeller := Labeller{
			ctx:         ctx,
			client:      client,
			gitLabelMap: nil,
			tags:        tags,
		}
		labeller.Label(owner, repo)
	},
}

func init() {
	rootCmd.AddCommand(labelCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// labelCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
}

func initGithubClient(ctx context.Context) *github.Client {
	githubAuth := os.Getenv("GITHUB_AUTH")
	if githubAuth == "" {
		panic(fmt.Errorf("No Github auth token provided"))
	}
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: githubAuth},
	)
	tc := oauth2.NewClient(ctx, ts)
	return github.NewClient(tc)
}

// Labeller labels Github PRs by matching PR commit titles to from a defined
// list of tags.
type Labeller struct {
	ctx         context.Context
	client      *github.Client
	gitLabelMap map[string]string
	tags        []string
}

type unlabelledPRs struct {
	unlabelledMu sync.RWMutex
	prs          []*github.Issue
}

// NewLabeller returns a new Github PR Labeller.
func NewLabeller(ctx context.Context, client *github.Client, tags []string) Labeller {

	return Labeller{
		ctx:         ctx,
		client:      client,
		gitLabelMap: nil,
		tags:        tags,
	}
}

// parseRepoPRs goes through the repo PRs and sends them through
// the provided chan.
func (l Labeller) parseRepoPRs(owner string, repo string, issueChan chan []*github.Issue) {
	pageToParse := 1
	prsExist := true

	for prsExist {
		issues, _, _ := l.client.Issues.ListByRepo(l.ctx, owner, repo, &github.IssueListByRepoOptions{
			State: "closed",
			ListOptions: github.ListOptions{
				Page:    pageToParse,
				PerPage: 100,
			},
		})

		if len(issues) > 0 {
			issueChan <- issues
			pageToParse++
		} else {
			prsExist = false
		}
	}
	close(issueChan)
}

func (l Labeller) matchWithTag(title string) (tag string) {
	for _, tag := range l.tags {
		lowerTitle := strings.ToLower(title)
		matched, _ := regexp.Match(tag, []byte(lowerTitle))
		if matched {
			return tag
		}
	}
	return "unknown"
}

func (l Labeller) mapGitLabels(owner string, repo string) map[string]string {
	gitLabels, _, _ := l.client.Issues.ListLabels(l.ctx, owner, repo, nil)
	var gitLabelMap = make(map[string]string)

	for _, gitLabel := range gitLabels {
		matchedLabel := l.matchWithTag(*gitLabel.Name)
		gitLabelMap[matchedLabel] = *gitLabel.Name
	}

	return gitLabelMap
}

func (l Labeller) applyLabelToIssue(owner string, repo string, issue *github.Issue) error {
	tag := l.matchWithTag(*issue.Title)

	if tag != "unknown" {
		l.client.Issues.Edit(l.ctx, owner, repo, *issue.Number, &github.IssueRequest{
			Labels: &[]string{l.gitLabelMap[tag]},
			// Labels: &[]string{},
		})
		return nil
	}
	return fmt.Errorf("No matching tag found")
}

// Label labels Github PRs by matching the PR title to a predefined list of tags.
// The tag is associated with a PR label, which is then applied to the PR.
func (l Labeller) Label(owner string, repo string) {
	var wg sync.WaitGroup
	var totalLabelledPRs uint64

	now := time.Now().UTC()
	issueChan := make(chan []*github.Issue)
	failedPRs := unlabelledPRs{
		prs: []*github.Issue{},
	}

	l.gitLabelMap = l.mapGitLabels(owner, repo)
	go l.parseRepoPRs(owner, repo, issueChan)
	for issues := range issueChan {
		for _, issue := range issues {
			if issue.IsPullRequest() {
				wg.Add(1)
				go func(issue *github.Issue) {
					defer wg.Done()
					err := l.applyLabelToIssue(owner, repo, issue)

					if err != nil {
						failedPRs.unlabelledMu.Lock()
						defer failedPRs.unlabelledMu.Unlock()

						failedPRs.prs = append(failedPRs.prs, issue)
					} else {
						atomic.AddUint64(&totalLabelledPRs, 1)
					}
				}(issue)
			}
		}
	}
	wg.Wait()

	fmt.Printf("Finished adding %d labels to PRs in %v \n", atomic.LoadUint64(&totalLabelledPRs), time.Since(now))
	fmt.Println("Unable to label the following PRs: ")
	for _, issue := range failedPRs.prs {
		fmt.Printf("%s - %s \n", *issue.Title, *issue.User.Login)
	}
}
