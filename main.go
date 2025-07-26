package main

import (
	"log"
	"net/http"

	"os"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/benraz123/gonotes/server"
)

var CLI struct {
	All         bool     `short:"a" name:"all" help:"show all"`
	Address     string   `short:"A" name:"address" help:"what adress to use (ADDRESS[:PORT])" default:"localhost:8080"`
	Directories []string `short:"d" name:"directories" help:"directories to broadcast from"`
}

func getDirs() []string {
	if CLI.Directories != nil {
		return CLI.Directories
	}

	cwd, err := os.Getwd()

	if err != nil {
		log.Fatalf("error getting working directory: %s", err)
	}

	return []string{cwd}
}

func getAddress() string {
	if strings.Contains(CLI.Address, ":") {
		return CLI.Address
	}

	return CLI.Address + ":8080"
}

func main() {
	kong.Parse(&CLI)

	dirs := getDirs()
	address := getAddress()

	http.Handle("/", server.New(CLI.All, dirs...))
	err := http.ListenAndServe(address, nil)
	log.Fatal(err)
}
