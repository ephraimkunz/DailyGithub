package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

type Test struct {
	PersistChanges bool `json:"persistChanges,omitempty"`
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		decoder := json.NewDecoder(r.Body)
		var t Test
		err := decoder.Decode(&t)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("Request:\n\tbody: %+v\n\theaders: %v", t, r.Header)

		w.Header().Set("Content-Type", "application/json")
		test := getResponse()
		item, err := json.Marshal(test)
		w.Write(item)
		return
	}

	fmt.Fprintf(w, "Hello, world")
}

func getResponse() map[string]interface{} {
	test := make(map[string]interface{})
	const RESP = "THIS IS FINALLY WORKING"
	test["speech"] = RESP
	test["displayText"] = RESP
	return test
}

func main() {
	http.HandleFunc("/", rootHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
