package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"strconv"
	"time"

	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/urlfetch"
)

func init() {
	http.HandleFunc("/", assistantHandler)
	http.HandleFunc("/authorize", assistantAuth)
}

func assistantHandler(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)

	if r.Method == http.MethodPost {
		b, err := httputil.DumpRequest(r, true)
		debug(ctx, b, err)
		decoder := json.NewDecoder(r.Body)
		fulfillmentReq := &FulfillmentReq{}
		err = decoder.Decode(fulfillmentReq)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		var builder FulfillmentBuilder
		switch fulfillmentReq.Result.Action {
		case SummaryIntent:
			builder, err = getProfileSummary(ctx, fulfillmentReq.OriginalRequest.Data.User.AccessToken)
		case TrendingReposIntent:
			ctxWithDeadline, _ := context.WithTimeout(ctx, 20*time.Second) // This call sometimes takes a while
			client := urlfetch.Client(ctxWithDeadline)
			if i, err := strconv.Atoi(fulfillmentReq.Result.Parameters.Number); err == nil && i != 0 {
				builder, err = getTrending(ctx, client, &i, extractLang(client, fulfillmentReq.Result.Parameters.Lang))
			} else {
				builder, err = getTrending(ctx, client, nil, extractLang(client, fulfillmentReq.Result.Parameters.Lang))
			}
		case NotificationsIntent:
			builder, err = getNotifications(ctx, fulfillmentReq.OriginalRequest.Data.User.AccessToken)
		case AssignedIssuesIntent:
			builder, err = getAssignedIssues(ctx, fulfillmentReq.OriginalRequest.Data.User.AccessToken)
		default:
			http.Error(w, "Incorrect fullfillment action", http.StatusInternalServerError)
			return
		}

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		fulfillmentResp := builder.buildFulfillment(ctx)

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

/*
Google forces us to own the endpoints we do oauth with. So we'll just proxy them to Github.
https://stackoverflow.com/questions/44288981/how-to-authenticate-user-with-just-a-google-account-on-actions-on-google
*/
func assistantAuth(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)

	b, err := httputil.DumpRequest(r, true)
	debug(ctx, b, err)
	url := "https://github.com/login/oauth/authorize" + "?" + r.URL.RawQuery
	log.Debugf(ctx, url)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}
