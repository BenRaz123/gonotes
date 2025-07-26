package main

import (
	"log"
	"net/http"

	"github.com/benraz123/gonotes/server"
)

func main() {
	http.Handle("/", server.New("/notes", "/home/ben/tmp/notes", "/w" /* sample values */))
    err := http.ListenAndServe("0.0.0.0:8080", nil)
	log.Fatal(err)
}
