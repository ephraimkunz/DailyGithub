package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"strconv"
)

func assistantHandler(w http.ResponseWriter, r *http.Request) {
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
				builder, err = getTrending(&i, extractLang(fulfillmentReq.Result.Parameters.Lang))
			} else {
				builder, err = getTrending(nil, extractLang(fulfillmentReq.Result.Parameters.Lang))
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