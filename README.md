# DailyGithub 
[![Build Status](https://travis-ci.org/ephraimkunz/DailyGithub.svg?branch=master)](https://travis-ci.org/ephraimkunz/DailyGithub)
[![Go Report Card](https://goreportcard.com/badge/github.com/ephraimkunz/DailyGithub)](https://goreportcard.com/report/github.com/ephraimkunz/DailyGithub)
Get your daily Github update from Google Assistant

## Testing
Run `go test` inside the directory root.

## Deployment
### Local
Run `go run dailygithub.go` in the directory root. The development server is at `http://localhost:8080/`. 
### Google App Engine
Run `gcloud app deploy` to create a docker container with the app and launch on GAE.
