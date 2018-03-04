package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"
	"time"

	"google.golang.org/appengine"
	"google.golang.org/appengine/urlfetch"
)

func init() {
	http.HandleFunc("/alexa", alexaHandler)
	http.HandleFunc("/token", alexaTokenProxyHandler)
}

const (
	// Required in json response
	AlexaVersion = "1.0"

	// Alexa intent types
	AlexaIntentTypeIntent       = "IntentRequest"
	AlexaIntentTypeLaunch       = "LaunchRequest"
	AlexaIntentTypeSessionEnded = "SessionEndedRequest"

	// Force a link account card to appear in the Alexa app
	AlexaCardTypeLink = "LinkAccount"

	// Alexa built-in intents we must handle
	AlexaHelpIntent   = "AMAZON.HelpIntent"
	AlexaCancelIntent = "AMAZON.CancelIntent"
	AlexaStopIntent   = "AMAZON.StopIntent"

	// SSML speech constants
	HelpText         = "<speak>You can ask for a summary of your Github profile, a list of trending repos, a list of your notifications, or a list of issues assigned to you.</speak>"
	WelcomeText      = "<speak>Welcome to DailyGithub! Let's get started. Ask for a summary of your Github profile, a list of trending repos, a list of your notifications, or a list of issues assigned to you.</speak>"
	AuthRequiredText = "<speak>This task requires linking your Github account to this skill.</speak>"
)

// Make a string have the buildFulfillment method
type AlexaStringResponse string

func (strResp *AlexaStringResponse) buildFulfillment(ctx context.Context) *FulfillmentResp {
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
	OutputSpeech     AlexaOutputSpeech `json:"outputSpeech,omitempty"`
	Card             *AlexaCard        `json:"card,omitempty"` // Pointer here to omit "card:{}" when empty struct
	ShouldEndSession bool              `json:"shouldEndSession"`
}

type AlexaCard struct {
	Type string `json:"type,omitempty"`
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
	ar.Response.ShouldEndSession = true
	return ar
}

func requiresAccessToken(name string) bool {
	return name == SummaryIntent ||
		name == NotificationsIntent ||
		name == AssignedIssuesIntent
}

func alexaHandler(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)

	validateRequest(ctx, w, r)
	switch r.Method {
	case http.MethodPost:
		b, err := httputil.DumpRequest(r, true)
		debug(ctx, b, err)

		decoder := json.NewDecoder(r.Body)
		alexaReq := &AlexaRequest{}
		err = decoder.Decode(alexaReq)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		var builder FulfillmentBuilder

		// This is how Alexa handles the "welcome" intent
		if alexaReq.Request.Type == AlexaIntentTypeLaunch {
			resp := NewAlexaResponse(WelcomeText)
			resp.Response.ShouldEndSession = false
			jsonResp, err := json.Marshal(resp)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Write(jsonResp)
			return
		}

		// Make sure that access_token is valid if invoking an intent requiring an access token
		if alexaReq.Session.User.AccessToken == "" && requiresAccessToken(alexaReq.Request.Intent.Name) {
			resp := NewAlexaResponse(AuthRequiredText)
			card := AlexaCard{AlexaCardTypeLink}
			resp.Response.Card = &card
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
			builder, err = getProfileSummary(ctx, alexaReq.Session.User.AccessToken)
		case TrendingReposIntent:
			ctxWithDeadline, _ := context.WithTimeout(ctx, 10*time.Second) // This call sometimes takes a while
			client := urlfetch.Client(ctxWithDeadline)
			if i, err := strconv.Atoi(alexaReq.Request.Intent.Slots.Number.Value); err == nil && i != 0 {
				builder, err = getTrending(ctx, client, &i, extractLang(client, alexaReq.Request.Intent.Slots.Lang.Value))
			} else {
				builder, err = getTrending(ctx, client, nil, extractLang(client, alexaReq.Request.Intent.Slots.Lang.Value))
			}
		case NotificationsIntent:
			builder, err = getNotifications(ctx, alexaReq.Session.User.AccessToken)
		case AssignedIssuesIntent:
			builder, err = getAssignedIssues(ctx, alexaReq.Session.User.AccessToken)
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

		str := builder.buildFulfillment(ctx).Speech
		str = strings.Replace(str, "&", "and", -1) // Alexa won't read ssml with '&' in it

		alexaResp := NewAlexaResponse(str)
		if alexaReq.Request.Intent.Name == AlexaHelpIntent {
			alexaResp.Response.ShouldEndSession = false // Session does not end on Launch intent or Help intent
		}

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

// Proxies the get token POST call part of the oauth flow to Github. This is needed because
// when Alexa requests the token, it doesn't have the Accept: application/json header, causing
// Github to return the response as a query string. Alexa expects it as JSON. So we do the call against
// Github after setting the Header, then respond to Alexa with the JSON body we received. Note: Github also
// expects the client_secret as a query parameter, not a Authorization: Basic <> encoded header.
// So make sure the checkbox for Credential Authentication Scheme is set to "Credentials in
// request body" in the Alexa setup console.
// https://forums.developer.amazon.com/articles/38610/alexa-debugging-account-linking.html

// Google makes us use this proxy, but for a different reason. They always pass credentials in the request body.
// They just want us to own the /token endpoint, so we proxy them to Github too.
func alexaTokenProxyHandler(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)

	b, err := httputil.DumpRequest(r, true)
	debug(ctx, b, err)

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Bad body", http.StatusBadRequest)
		return
	}

	newReq, err := http.NewRequest("POST", "https://github.com/login/oauth/access_token", bytes.NewBuffer(body))
	newReq.Header.Set("Accept", "application/json")

	client := urlfetch.Client(ctx)
	resp, err := client.Do(newReq)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	recievedBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Bad response from Github", http.StatusInternalServerError)
		return
	}

	w.Write(recievedBody)
}
