# DailyGithub 
[![Build Status](https://travis-ci.org/ephraimkunz/DailyGithub.svg?branch=master)](https://travis-ci.org/ephraimkunz/DailyGithub)
[![Go Report Card](https://goreportcard.com/badge/github.com/ephraimkunz/DailyGithub)](https://goreportcard.com/report/github.com/ephraimkunz/DailyGithub)

Get your daily Github update from Google Assistant / Amazon Alexa

## Live Versions
Amazon Alexa skill is [here](https://www.amazon.com/dp/B078LHPJWM/ref=sr_1_13?s=digital-skills&ie=UTF8&qid=1514383804&sr=1-13&keywords=github).

Google Assistant skill is [here](https://assistant.google.com/services/a/uid/000000c1a6473f2d?hl=en).

## Things to ask
* "Get issues assigned to me."  "My assigned issues."
* "Get my notifications." "Read notifications."
* "Profile summary." "Github profile summary."
* "Trending repos." "Top repos in Golang." "Top 6 trending repos in Javascript."

## Testing
Run `go test` inside the directory root.

## Deployment
### Local
Run `go run dailygithub.go` in the directory root. The development server is at `http://localhost:8080/`. 
### Google App Engine
Run `gcloud app deploy` to create a docker container with the app and launch on GAE.

