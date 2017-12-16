package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

type Test struct {
	PersistChanges bool `json:"persistChanges,omitempty"`
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Headers: %v", r.Header)

	if r.Method == "POST" {
		b, err := ioutil.ReadAll(r.Body)
		defer r.Body.Close()
		log.Printf("Body: %s", string(b))

		w.Header().Set("Content-Type", "application/json")
		test := getResponse()
		item, err := json.Marshal(test)
		if err != nil {
			http.Error(w, "Error marshaling json", http.StatusInternalServerError)
		}

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
