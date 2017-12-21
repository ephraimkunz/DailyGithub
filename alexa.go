package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"strconv"
)

const AlexaVersion = "1.0"

type AlexaRequest struct {
	Session AlexaSession        `json:"session,omitempty"`
	Request AlexaRequestDetails `json:"request,omitempty"`
}

type AlexaRequestDetails struct {
	Intent AlexaIntent `json:"intent,omitempty"`
}

type AlexaIntent struct {
	Name  string     `json:"name,omitempty"`
	Slots AlexaSlots `json:"slots,omitempty"`
}

type AlexaSlots struct {
	Number string `json:"number,omitempty"`
	Lang   string `json:"lang,omitempty"`
}

type AlexaSession struct {
	User AlexaUser `json:"user,omitempty"`
}

type AlexaUser struct {
	AccessToken string `json:"accessToken,omitempty"`
}

type AlexaResponse struct {
	Version  string               `json:"version,omitempty"`
	Response AlexaResponseDetails `json:"response,omitempty"`
}

type AlexaResponseDetails struct {
	OutputSpeech AlexaOutputSpeech `json:"outputSpeech,omitempty"`
}

type AlexaOutputSpeech struct {
	Type string `json:"type,omitempty"`
	SSML string `json:"ssml,omitempty"`
}

func NewAlexaResponse(ssml string) AlexaResponse {
	ar := AlexaResponse{}
	ar.Version = AlexaVersion
	ar.Response.OutputSpeech.Type = "SSML"
	ar.Response.OutputSpeech.SSML = ssml
	return ar
}

func alexaHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		debug(httputil.DumpRequest(r, true))
		decoder := json.NewDecoder(r.Body)
		alexaReq := &AlexaRequest{}
		err := decoder.Decode(alexaReq)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		var builder FulfillmentBuilder

		switch alexaReq.Request.Intent.Name {
		case SummaryIntent:
			builder, err = getProfileSummary(alexaReq.Session.User.AccessToken)
		case TrendingReposIntent:

			if i, err := strconv.Atoi(alexaReq.Request.Intent.Slots.Number); err == nil && i != 0 {
				builder, err = getTrending(&i, extractLang(alexaReq.Request.Intent.Slots.Lang))
			} else {
				builder, err = getTrending(nil, extractLang(alexaReq.Request.Intent.Slots.Lang))
			}
		case NotificationsIntent:
			builder, err = getNotifications(alexaReq.Session.User.AccessToken)
		case AssignedIssuesIntent:
			builder, err = getAssignedIssues(alexaReq.Session.User.AccessToken)
		default:
			http.Error(w, "Incorrect fullfillment action", http.StatusInternalServerError)
			return
		}

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		fulfillmentResp := builder.buildFulfillment()

		alexaResp := NewAlexaResponse(fulfillmentResp.Speech)

		resp, err := json.Marshal(alexaResp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Write(resp)
	case http.MethodGet:
		fmt.Fprint(w, "Hello, world alexa")
	}
}
