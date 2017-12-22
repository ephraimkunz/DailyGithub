package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"
)

const (
	AlexaVersion = "1.0"

	AlexaIntentTypeIntent       = "IntentRequest"
	AlexaIntentTypeLaunch       = "LaunchRequest"
	AlexaIntentTypeSessionEnded = "SessionEndedRequest"

	AlexaHelpIntent   = "AMAZON.HelpIntent"
	AlexaCancelIntent = "AMAZON.CancelIntent"
	AlexaStopIntent   = "AMAZON.StopIntent"

	HelpText    = "<speak>You can ask for a summary of your Github profile, a list of trending repos, a list of your notifications, or a list of issues assigned to you.</speak>"
	WelcomeText = "<speak>Welcome to DailyGithub! Let's get started. Ask for a summary of your Github profile, a list of trending repos, a list of your notifications, or a list of issues assigned to you.</speak>"
)

// Make a string have the buildFulfillment method
type AlexaStringResponse string

func (strResp *AlexaStringResponse) buildFulfillment() *FulfillmentResp {
	str := string(*strResp)
	fr := &FulfillmentResp{str, str}
	return fr
}

type AlexaRequest struct {
	Session AlexaSession        `json:"session,omitempty"`
	Request AlexaRequestDetails `json:"request,omitempty"`
}

type AlexaRequestDetails struct {
	Type   string      `json:"type,omitempty"`
	Intent AlexaIntent `json:"intent,omitempty"`
}

type AlexaIntent struct {
	Name  string     `json:"name,omitempty"`
	Slots AlexaSlots `json:"slots,omitempty"`
}

type AlexaSlots struct {
	Number AlexaSlot `json:"number,omitempty"`
	Lang   AlexaSlot `json:"lang,omitempty"`
}

type AlexaSlot struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
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
	Type string `json:"type"`
	SSML string `json:"ssml"`
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

		// This is how Alexa handles the "welcome" intent
		if alexaReq.Request.Type == AlexaIntentTypeLaunch {
			resp := NewAlexaResponse(WelcomeText)
			jsonResp, err := json.Marshal(resp)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Write(jsonResp)
			return
		}

		switch alexaReq.Request.Intent.Name {
		case SummaryIntent:
			builder, err = getProfileSummary(alexaReq.Session.User.AccessToken)
		case TrendingReposIntent:

			if i, err := strconv.Atoi(alexaReq.Request.Intent.Slots.Number.Value); err == nil && i != 0 {
				builder, err = getTrending(&i, extractLang(alexaReq.Request.Intent.Slots.Lang.Value))
			} else {
				builder, err = getTrending(nil, extractLang(alexaReq.Request.Intent.Slots.Lang.Value))
			}
		case NotificationsIntent:
			builder, err = getNotifications(alexaReq.Session.User.AccessToken)
		case AssignedIssuesIntent:
			builder, err = getAssignedIssues(alexaReq.Session.User.AccessToken)
		case AlexaHelpIntent:
			resp := AlexaStringResponse(HelpText)
			builder, err = &resp, nil
		case AlexaCancelIntent, AlexaStopIntent:
			resp := AlexaStringResponse("") // Just stop whatever is going on
			builder, err = &resp, nil
		default:
			http.Error(w, "Incorrect fullfillment action", http.StatusInternalServerError)
			return
		}

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		str := builder.buildFulfillment().Speech
		str = strings.Replace(str, "&", "and", -1) // Alexa won't read ssml with '&' in it

		alexaResp := NewAlexaResponse(str)

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
