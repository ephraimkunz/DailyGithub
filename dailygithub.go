package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	_ "net/http/pprof"

	"github.com/ephraimkunz/go-trending"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/urlfetch"
)

const (
	SummaryIntent        = "summary_intent"
	TrendingReposIntent  = "trending_repos_intent"
	NotificationsIntent  = "notifications_intent"
	AssignedIssuesIntent = "assigned_issues_intent"
	defaultTrendingRepos = 5
)

// Create new types so we can make them conform to FulfillmentBuilder
type GithubNotifications []*github.Notification
type GithubIssues []*github.Issue

type ProfileSummary struct {
	user *github.User
}

type FulfillmentReq struct {
	OriginalRequest OriginalReq `json:"originalRequest,omitempty"`
	Result          ResultReq   `json:"result,omitempty"`
}

type ResultReq struct {
	Action     string        `json:"action,omitempty"`
	Parameters ParametersReq `json:"parameters,omitempty"`
}

type ParametersReq struct {
	Number string `json:"number,omitempty"`
	Lang   string `json:"lang,omitempty"`
}

type OriginalReq struct {
	Data DataReq `json:"data,omitempty"`
}

type DataReq struct {
	User UserReq `json:"user,omitempty"`
}

type FulfillmentResp struct {
	Speech      string `json:"speech,omitempty"`
	DisplayText string `json:"displayText,omitempty"`
}

type UserReq struct {
	LastSeen    string `json:"lastSeen,omitempty"`
	AccessToken string `json:"accessToken,omitempty"`
	Locale      string `json:"locale,omitempty"`
	UserId      string `json:"userId,omitempty"`
}

type Trending struct {
	text, speech string
}

type FulfillmentBuilder interface {
	buildFulfillment(ctx context.Context) *FulfillmentResp
}

func minInt(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func extractLang(client *http.Client, lang string) string {
	trend := trending.NewTrendingWithClient(client)
	langs, err := trend.GetLanguages()
	if err != nil {
		return ""
	}

	if lang == "" {
		return ""
	}

	for _, trendLang := range langs {
		if strings.ToLower(trendLang.Name) == strings.ToLower(lang) {
			return trendLang.URLName
		}
	}

	return ""
}

func debug(ctx context.Context, data []byte, err error) {
	if err == nil {
		log.Debugf(ctx, "Request", string(data))
	} else {
		log.Debugf(ctx, err.Error())
	}
}

// Count may be nil if the user didn't specify how many. Give them the default value.
func getTrending(ctx context.Context, client *http.Client, count *int, lang string) (FulfillmentBuilder, error) {
	projects, err := get(ctx, lang)
	if err != nil {
		return nil, err
	}

	var projectText, projectSpeech string
	var maxTrending int
	if count == nil {
		maxTrending = defaultTrendingRepos
	} else {
		maxTrending = *count
	}

	if lang != "" {
		projectSpeech = fmt.Sprintf("<p>Here are the top %d trending repositories for %s:</p>", minInt(len(projects.Data), maxTrending), lang)
	} else {
		projectSpeech = fmt.Sprintf("<p>Here are the top %d trending repositories:</p>", minInt(len(projects.Data), maxTrending))
	}

	for index, project := range projects.Data {
		if index >= maxTrending {
			break
		}
		projectSpeech += fmt.Sprintf("<p>#%d. %s by %s: %s</p>", index+1, project.RepositoryName, project.Owner, project.Description)
		projectText += fmt.Sprintf("\n#%d. %s by %s: %s", index+1, project.RepositoryName, project.Owner, project.Description)
	}

	return &Trending{projectText, projectSpeech}, nil
}

func (sum *ProfileSummary) buildFulfillment(ctx context.Context) *FulfillmentResp {
	summary := fmt.Sprintf(
		"Hello %s. You currently have %d public repos, "+
			"%d private repos, and you own %d of these private repos."+
			"You have %d followers and are following %d people.",
		sum.user.GetName(), sum.user.GetPublicRepos(), sum.user.GetTotalPrivateRepos(),
		sum.user.GetOwnedPrivateRepos(), sum.user.GetFollowers(), sum.user.GetFollowing())
	resp := &FulfillmentResp{Speech: "<speak>" + summary + "</speak>", DisplayText: summary}
	log.Debugf(ctx, "Built fulfillment with string ", summary)
	return resp
}

func (trending *Trending) buildFulfillment(ctx context.Context) *FulfillmentResp {
	resp := &FulfillmentResp{Speech: "<speak>" + trending.speech + "</speak>", DisplayText: trending.text}
	log.Debugf(ctx, "Built fulfillment with string ", trending.speech)
	return resp
}

func (not *GithubNotifications) buildFulfillment(ctx context.Context) *FulfillmentResp {
	var text, speech string
	if len([]*github.Notification(*not)) > 0 {
		speech = "<speak><p>Here are your unread notifications:</p>"
	} else {
		speech = "<speak>You have no unread notifications"
	}

	for i, notification := range []*github.Notification(*not) {
		text += fmt.Sprintf("\n#%d: This notification is on an %s and says: %s", i+1, notification.Subject.GetType(), notification.Subject.GetTitle())
		speech += fmt.Sprintf("<p>#%d: This notification is on an %s and says: %s</p>", i+1, notification.Subject.GetType(), notification.Subject.GetTitle())
	}
	return &FulfillmentResp{speech + "</speak>", text}
}

func (iss *GithubIssues) buildFulfillment(ctx context.Context) *FulfillmentResp {
	var text, speech string
	if len([]*github.Issue(*iss)) > 0 {
		speech = fmt.Sprintf("<speak><p>Here are the open issues assigned to you:</p>")
	} else {
		speech = fmt.Sprintf("<speak>You have no open issues assigned to you.")
	}

	for i, issue := range []*github.Issue(*iss) {
		text += fmt.Sprintf("\n#%d: Opened in %s on %s by %s: %s", i+1, issue.Repository.GetName(), issue.GetCreatedAt().Format("Monday, January 2"), issue.User.GetLogin(), issue.GetTitle())
		speech += fmt.Sprintf("<p>#%d: Opened in %s on %s by %s: %s</p>", i+1, issue.Repository.GetName(), issue.GetCreatedAt().Format("Monday, January 2"), issue.User.GetLogin(), issue.GetTitle())
	}
	return &FulfillmentResp{speech + "</speak>", text}
}

func getNotifications(ctx context.Context, accessToken string) (FulfillmentBuilder, error) {
	client := createGithubClient(ctx, accessToken)
	notifications, _, err := client.Activity.ListNotifications(ctx, nil)

	if err != nil {
		return nil, err
	}

	ghNot := GithubNotifications(notifications)
	return &ghNot, nil
}

func getAssignedIssues(ctx context.Context, accessToken string) (FulfillmentBuilder, error) {
	client := createGithubClient(ctx, accessToken)
	issues, _, err := client.Issues.List(ctx, true, nil)

	if err != nil {
		return nil, err
	}

	iss := GithubIssues(issues)
	return &iss, nil
}

func getProfileSummary(ctx context.Context, accessToken string) (FulfillmentBuilder, error) {
	client := createGithubClient(ctx, accessToken)
	user, _, err := client.Users.Get(ctx, "") // Get authenticated user

	if err != nil {
		return nil, err
	}

	summary := &ProfileSummary{user} // Type conversion to custom type so we can use buildFulfillment method

	return summary, nil
}

func createGithubClient(ctx context.Context, accessToken string) *github.Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: accessToken})
	authClient := &http.Client{
		Transport: &oauth2.Transport{
			Source: oauth2.ReuseTokenSource(nil, ts),
			Base:   &urlfetch.Transport{Context: ctx},
		},
	}
	client := github.NewClient(authClient)
	return client
}
