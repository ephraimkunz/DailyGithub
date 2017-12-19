package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	_ "net/http/pprof"
	"strconv"

	"github.com/andygrunwald/go-trending"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

const (
	SummaryIntent        = "input.summary"
	TrendingReposIntent  = "input.trending"
	NotificationsIntent  = "input.notifications"
	AssignedIssuesIntent = "input.assigned_issues"
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

func rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		debug(httputil.DumpRequest(r, true))
		decoder := json.NewDecoder(r.Body)
		fulfillmentReq := &FulfillmentReq{}
		err := decoder.Decode(fulfillmentReq)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		var builder FulfillmentBuilder

		switch fulfillmentReq.Result.Action {
		case SummaryIntent:
			builder, err = getProfileSummary(fulfillmentReq.OriginalRequest.Data.User.AccessToken)
		case TrendingReposIntent:
			if i, err := strconv.Atoi(fulfillmentReq.Result.Parameters.Number); err == nil && i != 0 {
				builder, err = getTrending(&i)
			} else {
				builder, err = getTrending(nil)
			}
		case NotificationsIntent:
			builder, err = getNotifications(fulfillmentReq.OriginalRequest.Data.User.AccessToken)
		case AssignedIssuesIntent:
			builder, err = getAssignedIssues(fulfillmentReq.OriginalRequest.Data.User.AccessToken)
		default:
			http.Error(w, "Incorrect fullfillment action", http.StatusInternalServerError)
			return
		}

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		fulfillmentResp := builder.buildFulfillment()

		resp, err := json.Marshal(fulfillmentResp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write(resp)
	} else if r.Method == http.MethodGet {
		fmt.Fprintf(w, "Hello, world")
	}
}

func debug(data []byte, err error) {
	if err == nil {
		log.Println("Request", string(data))
	} else {
		log.Println(err)
	}
}

// Count may be nil if the user didn't specify how many. Give them the default value.
func getTrending(count *int) (FulfillmentBuilder, error) {
	trend := trending.NewTrending()
	projects, err := trend.GetProjects(trending.TimeToday, "")
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

	projectSpeech = fmt.Sprintf("<p>Here are the top %d trending repositories:</p>", minInt(len(projects), maxTrending))
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
	resp := &FulfillmentResp{Speech: summary, DisplayText: summary}
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
	http.HandleFunc("/", rootHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
