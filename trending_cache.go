package main

import (
	"context"
	"encoding/json"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/ephraimkunz/go-trending"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/urlfetch"
)

type TrendingProjects struct {
	Data []trending.Project
}

type StorableJSON struct {
	Json string `datastore:",noindex"` // Must not index long strings
}

const allLanguagesKey = "all" // Use to store in Cloud Datastore

func init() {
	http.HandleFunc("/tasks/refreshTrendingCache", refreshTrendingCache)
	rand.Seed(time.Now().UnixNano())
}

func (tp *TrendingProjects) toStorableJSON(ctx context.Context) (*StorableJSON, error) {
	stringify, err := json.Marshal(tp)

	if err != nil {
		log.Debugf(ctx, "Error marshalling: %v", err)
		return nil, err
	}
	return &StorableJSON{string(stringify)}, nil
}

func fromJSON(storable StorableJSON) *TrendingProjects {
	proj := TrendingProjects{}
	json.Unmarshal([]byte(storable.Json), &proj)
	return &proj
}

func put(ctx context.Context, key string, val *TrendingProjects) error {
	if key == "" {
		key = allLanguagesKey
	}

	datastoreKey := datastore.NewKey(ctx, "TrendingProjects", key, 0, nil)
	js, err := val.toStorableJSON(ctx)
	if err != nil {
		return err
	}

	if _, err := datastore.Put(ctx, datastoreKey, js); err != nil {
		return err
	}

	return nil
}

func get(ctx context.Context, key string) (TrendingProjects, error) {
	if key == "" {
		key = allLanguagesKey
	}

	var sj StorableJSON
	datastoreKey := datastore.NewKey(ctx, "TrendingProjects", key, 0, nil)
	err := datastore.Get(ctx, datastoreKey, &sj)

	projects := fromJSON(sj)

	return *projects, err
}

// Cache trending data in Cloud Datastore because Github's trending endpoint
// is so slow that Google Assistant times out before receiving a response.
func refreshTrendingCache(w http.ResponseWriter, r *http.Request) {
	ctx := appengine.NewContext(r)
	ctxWithDeadline, _ := context.WithTimeout(ctx, 1*time.Hour) // This call sometimes takes a while
	client := urlfetch.Client(ctxWithDeadline)
	trend := trending.NewTrendingWithClient(client)
	languages, err := trend.GetLanguages()

	if err != nil {
		log.Errorf(ctxWithDeadline, "Failed to fetch languages for trending cache: %v", err)
		http.Error(w, "Failed to fetch languages", http.StatusInternalServerError)
		return
	}

	languages = append(languages,
		trending.Language{
			Name:    "",
			URLName: "",
			URL:     nil,
		}) // Fetch for any language

	log.Infof(ctxWithDeadline, "Num languages: %d", len(languages))

	for i := range languages {
		j := rand.Intn(i + 1)
		languages[i], languages[j] = languages[j], languages[i]
	}

	const requestWindow = 20
	const avgRequestsPerSecond = 0.5

	limit := make(chan struct{}, requestWindow) // Number of concurrent requests
	var wg sync.WaitGroup

	wg.Add(len(languages))

	for _, language := range languages {
		limit <- struct{}{}

		go func(lang string) {
			defer wg.Done()

			secs := rand.Intn(requestWindow / avgRequestsPerSecond)
			duration := time.Duration(secs) * time.Second
			time.Sleep(duration)

			projects, err := trend.GetProjects(trending.TimeToday, lang)
			log.Infof(ctxWithDeadline, "Fetched %s: %d", lang, len(projects))
			if err != nil {
				log.Errorf(ctxWithDeadline, "Failed to fetch trending repos for %s: %v", lang, err)
				<-limit
				return
			}

			if len(projects) == 0 {
				projects = make([]trending.Project, 0) // Don't want to serialize to null in JSON

				// If there's already data out there, even if old, don't replace with empty slice
				if proj, err := get(ctxWithDeadline, lang); err != nil && len(proj.Data) > 0 {
					<-limit
					return
				}
			}

			if err := put(ctxWithDeadline, lang, &TrendingProjects{projects}); err != nil {
				log.Errorf(ctxWithDeadline, "Failed to store val %v for key %v: %v", projects, lang, err)
			}
			<-limit
		}(language.URLName)
	}

	wg.Wait() // Don't let context go out of scope
}

// Called when app first deployed to GAE
func warmup(w http.ResponseWriter, r *http.Request) {
	refreshTrendingCache(w, r)
}
