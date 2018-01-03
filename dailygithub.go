package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"strings"

	"github.com/andygrunwald/go-trending"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
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
	buildFulfillment() *FulfillmentResp
}

func minInt(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func extractLang(lang string) string {
	trend := trending.NewTrending()
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

func debug(data []byte, err error) {
	if err == nil {
		log.Println("Request", string(data))
	} else {
		log.Println(err)
	}
}

// Count may be nil if the user didn't specify how many. Give them the default value.
func getTrending(count *int, lang string) (FulfillmentBuilder, error) {
	trend := trending.NewTrending()
	projects, err := trend.GetProjects(trending.TimeToday, lang)
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
		projectSpeech = fmt.Sprintf("<p>Here are the top %d trending repositories for %s:</p>", minInt(len(projects), maxTrending), lang)
	} else {
		projectSpeech = fmt.Sprintf("<p>Here are the top %d trending repositories:</p>", minInt(len(projects), maxTrending))
	}

	for index, project := range projects {
		if index >= maxTrending {
			break
		}
		projectSpeech += fmt.Sprintf("<p>#%d. %s by %s: %s</p>", index+1, project.RepositoryName, project.Owner, project.Description)
		projectText += fmt.Sprintf("\n#%d. %s by %s: %s", index+1, project.RepositoryName, project.Owner, project.Description)
	}

	return &Trending{projectText, projectSpeech}, nil
}

func (sum *ProfileSummary) buildFulfillment() *FulfillmentResp {
	summary := fmt.Sprintf(
		"Hello %s. You currently have %d public repos, "+
			"%d private repos, and you own %d of these private repos."+
			"You have %d followers and are following %d people.",
		sum.user.GetName(), sum.user.GetPublicRepos(), sum.user.GetTotalPrivateRepos(),
		sum.user.GetOwnedPrivateRepos(), sum.user.GetFollowers(), sum.user.GetFollowing())
	resp := &FulfillmentResp{Speech: "<speak>" + summary + "</speak>", DisplayText: summary}
	log.Println("Built fulfillment with string ", summary)
	return resp
}

func (trending *Trending) buildFulfillment() *FulfillmentResp {
	resp := &FulfillmentResp{Speech: "<speak>" + trending.speech + "</speak>", DisplayText: trending.text}
	log.Println("Built fulfillment with string ", trending.speech)
	return resp
}

func (not *GithubNotifications) buildFulfillment() *FulfillmentResp {
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

func (iss *GithubIssues) buildFulfillment() *FulfillmentResp {
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

func getNotifications(accessToken string) (FulfillmentBuilder, error) {
	client, ctx := createGithubClient(accessToken)
	notifications, _, err := client.Activity.ListNotifications(ctx, nil)

	if err != nil {
		return nil, err
	}

	ghNot := GithubNotifications(notifications)
	return &ghNot, nil
}

func getAssignedIssues(accessToken string) (FulfillmentBuilder, error) {
	client, ctx := createGithubClient(accessToken)
	issues, _, err := client.Issues.List(ctx, true, nil)

	if err != nil {
		return nil, err
	}

	iss := GithubIssues(issues)
	return &iss, nil
}

func getProfileSummary(accessToken string) (FulfillmentBuilder, error) {
	client, ctx := createGithubClient(accessToken)
	user, _, err := client.Users.Get(ctx, "") // Get authenticated user

	if err != nil {
		return nil, err
	}

	summary := &ProfileSummary{user} // Type conversion to custom type so we can use buildFulfillment method

	return summary, nil
}

func createGithubClient(accessToken string) (*github.Client, context.Context) {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: accessToken})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)
	return client, ctx
}

func main() {
	http.HandleFunc("/", assistantHandler)
	http.HandleFunc("/authorize", assistantAuth)
	http.HandleFunc("/alexa", alexaHandler)
	http.HandleFunc("/token", alexaTokenProxyHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
