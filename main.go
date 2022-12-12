package main

import (
	"context"
	"fmt"
	"os"

	"github.com/google/go-github/v48/github"
	"golang.org/x/oauth2"

	githubql "github.com/shurcooL/githubv4"
)

const (
	// perPage is the number of items to return per page.
	perPage = 100
	// orgName is the name of the GitHub organization to query.
	orgName = "kubernetes"
	// projectName is the name of the GitHub project to query.
	projectName = "@aramase's untitled project" // "SIG Auth"
	// columnName is the name of the GitHub project column to query.
	columnName = "Needs Triage"
)

type ghClient struct {
	*github.Client
	v4Client *githubql.Client
}

type projectColumn struct {
	ID   githubql.ID
	Name githubql.String
}

func main() {
	ctx := context.Background()
	token := os.Getenv("GITHUB_TOKEN")
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(context.Background(), ts)
	client := ghClient{Client: github.NewClient(tc), v4Client: githubql.NewClient(tc)}

	projectID, err := client.getProjectID(ctx, orgName, projectName)
	must(err)

	columnID, err := client.getProjectColumnID(ctx, projectID, columnName)
	must(err)

	fmt.Println("columnID: ", columnID)

	repos, err := client.listRepos(ctx, orgName)
	must(err)

	for _, repo := range repos {
		fmt.Printf("Looking for issues and PRs in %s/%s\n", orgName, *repo.Name)

		items, err := client.listIssuesAndPullRequests(ctx, orgName, *repo.Name, "sig/auth")
		must(err)

		for _, item := range items {
			// add pull request and issue to project
		}

		fmt.Printf("found %d in repo\n", len(items))
	}
}

func (c *ghClient) listRepos(ctx context.Context, org string) ([]*github.Repository, error) {
	var allRepos []*github.Repository
	opt := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: perPage},
	}

	for {
		repos, resp, err := c.Repositories.ListByOrg(ctx, org, opt)
		if err != nil {
			return nil, err
		}
		allRepos = append(allRepos, repos...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	return allRepos, nil
}

func (c *ghClient) listIssuesAndPullRequests(ctx context.Context, owner, repo string, labels ...string) ([]*github.Issue, error) {
	var allIssues []*github.Issue
	opts := &github.IssueListByRepoOptions{
		Labels: labels,
		ListOptions: github.ListOptions{
			PerPage: perPage,
		},
	}

	for {
		// Note: As far as the GitHub API is concerned, every pull request is an issue,
		// but not every issue is a pull request. Some endpoints, events, and webhooks
		// may also return pull requests via this struct. If PullRequestLinks is nil,
		// this is an issue, and if PullRequestLinks is not nil, this is a pull request.
		// The IsPullRequest helper method can be used to check that.
		// xref: https://docs.github.com/en/rest/issues/issues?apiVersion=2022-11-28#list-repository-issues
		issues, resp, err := c.Issues.ListByRepo(ctx, owner, repo, opts)
		if err != nil {
			return nil, err
		}
		allIssues = append(allIssues, issues...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allIssues, nil
}

func (c *ghClient) getProjectID(ctx context.Context, org, name string) (githubql.ID, error) {
	var query struct {
		Organization struct {
			ProjectV2 struct {
				Nodes []struct {
					ID    githubql.ID
					Title githubql.String
				}
			} `graphql:"projectsV2(first: 100)"`
		} `graphql:"organization(login: $org)"`
	}

	variables := map[string]interface{}{
		"org": githubql.String(org),
	}

	err := c.v4Client.Query(ctx, &query, variables)
	if err != nil {
		return nil, err
	}

	for _, project := range query.Organization.ProjectV2.Nodes {
		if project.Title == githubql.String(name) {
			fmt.Printf("found project %q with ID %q\n", name, project.ID)
			return project.ID, nil
		}
	}

	return nil, fmt.Errorf("project %q not found", name)
}

// func (c *ghClient) listProjectColumns(ctx context.Context, projectID githubql.ID) ([]*github.ProjectColumn, error) {
// 	var allColumns []*github.ProjectColumn
// 	opt := &github.ListOptions{PerPage: perPage}

// 	for {
// 		columns, resp, err := c.Projects.ListProjectColumns(ctx, projectID, opt)
// 		if err != nil {
// 			return nil, err
// 		}
// 		allColumns = append(allColumns, columns...)
// 		if resp.NextPage == 0 {
// 			break
// 		}
// 		opt.Page = resp.NextPage
// 	}

// 	return allColumns, nil
// }

func (c *ghClient) getProjectColumnID(ctx context.Context, projectID githubql.ID, name string) (githubql.ID, error) {
	// xref: https://docs.github.com/en/issues/planning-and-tracking-with-projects/automating-your-project/using-the-api-to-manage-projects#finding-the-node-id-of-a-field
	var query struct {
		Node struct {
			ProjectV2 struct {
				Fields struct {
					Nodes []struct {
						ProjectV2SingleSelectField struct {
							ID      githubql.ID
							Name    githubql.String
							Options []struct {
								ID   githubql.ID
								Name githubql.String
							}
						} `graphql:"... on ProjectV2SingleSelectField"`
					} `graphql:"nodes"`
				} `graphql:"fields(first: 20)"`
			} `graphql:"... on ProjectV2"`
		} `graphql:"node(id: $projectID)"`
	}

	variables := map[string]interface{}{
		"projectID": projectID,
	}

	err := c.v4Client.Query(ctx, &query, variables)
	if err != nil {
		return nil, err
	}

	for _, column := range query.Node.ProjectV2.Fields.Nodes {
		if column.ProjectV2SingleSelectField.Name != "Status" {
			continue
		}
		for _, option := range column.ProjectV2SingleSelectField.Options {
			if option.Name == githubql.String(name) {
				return option.ID, nil
			}
		}
	}

	return nil, fmt.Errorf("column %q not found", name)
}

func (c *ghClient) addPullRequestToProject(ctx context.Context, columnID int64, contentID int64, contentType string) error {
	return nil
}

func (c *ghClient) addIssueToProject(ctx context.Context, columnID int64, contentID int64, contentType string) error {
	return nil
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
