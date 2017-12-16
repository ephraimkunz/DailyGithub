package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/andygrunwald/go-trending"
)

type User struct {
	Name              string `json:"name,omitempty"`
	PublicRepos       int    `json:"public_repos,omitempty"`
	Followers         int    `json:"followers,omitempty"`
	Following         int    `json:"following,omitempty"`
	TotalPrivateRepos int    `json:"total_private_repos,omitempty"`
	OwnedPrivateRepos int    `json:"owned_private_repos,omitempty"`
}

type FulfillmentReq struct {
	OriginalRequest OriginalReq `json:"originalRequest,omitempty"`
	Result          ResultReq   `json:"result,omitempty"`
}

type ResultReq struct {
	Action string `json:"action,omitempty"`
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
	str string
}

type Fulfillment interface {
	buildFulfillment() *FulfillmentResp
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		decoder := json.NewDecoder(r.Body)
		fulfillmentReq := &FulfillmentReq{}
		err := decoder.Decode(fulfillmentReq)
		if err != nil {
			http.Error(w, "Error decoding json", http.StatusInternalServerError)
		}

		s, err := ioutil.ReadAll(r.Body)
		log.Printf("Body: %s", s)

		w.Header().Set("Content-Type", "application/json")

		var fulfillmentResp *FulfillmentResp

		if fulfillmentReq.Result.Action == "summary_intent" {
			user, err := getCurrentUser(fulfillmentReq.OriginalRequest.Data.User.AccessToken)
			if err != nil {
				log.Println(err)
			}

			fulfillmentResp = user.buildFulfillment()

		} else if fulfillmentReq.Result.Action == "hot_repo_intent" {
			hotRepos, err := getTrending()
			if err != nil {
				http.Error(w, "Error getting trending repos", http.StatusInternalServerError)
			}

			fulfillmentResp = hotRepos.buildFulfillment()
		}

		resp, err := json.Marshal(fulfillmentResp)
		if err != nil {
			http.Error(w, "Error marshaling json", http.StatusInternalServerError)
		}
		w.Write(resp)
		return
	}

	fmt.Fprintf(w, "Hello, world")
}

func getTrending() (*Trending, error) {
	trend := trending.NewTrending()
	projects, err := trend.GetProjects(trending.TimeToday, "")
	if err != nil {
		return nil, err
	}

	var projectText string

	for index, project := range projects {
		if index > 4 {
			break // Only take first 5
		}
		projectText += fmt.Sprintf("\n#%d. %s by %s: %s", index+1, project.Name, project.Owner, project.Description)
	}

	return &Trending{projectText}, nil
}

func (user *User) buildFulfillment() *FulfillmentResp {
	summary := fmt.Sprintf(
		"Hello %s. You currently have %d public repos, "+
			"%d private repos, and you own %d of these private repos."+
			"You have %d followers and are following %d people.",
		user.Name, user.PublicRepos, user.TotalPrivateRepos,
		user.OwnedPrivateRepos, user.Followers, user.Following)
	resp := &FulfillmentResp{Speech: summary, DisplayText: summary}
	return resp
}

func (trending *Trending) buildFulfillment() *FulfillmentResp {
	resp := &FulfillmentResp{Speech: trending.str, DisplayText: trending.str}
	return resp
}

func getCurrentUser(accessToken string) (*User, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)

	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "token "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)

	if err != nil {
		return nil, err
	}

	decoder := json.NewDecoder(resp.Body)
	user := &User{}

	err = decoder.Decode(user)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func main() {
	http.HandleFunc("/", rootHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
