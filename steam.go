package main

import (
	"flag"
	"fmt"
	//"html"
	"encoding/json"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

var (
	password = flag.String("password", "", "Password to use")
	filename = flag.String("file", "", "Name for the app list file")
)

func createRequest(password string, app int) *http.Request {
	u, _ := url.Parse("http://store.steampowered.com/actions/clues")

	query := u.Query()
	query.Set("key", password)
	query.Set("_", strconv.Itoa(rand.Int()))
	rand.Seed(time.Now().UnixNano())
	u.RawQuery = query.Encode()

	r := &http.Request{
		Method: "GET",
		URL:    u,
	}

	r.Header = http.Header{"Referer": []string{fmt.Sprintf("http://store.steampowered.com/app/%d/", app)}}

	return r
}

func rateLimited() bool {
	req := createRequest("94050999014715", 6900)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	return resp.StatusCode != 200
}

func checkResponse(resp *http.Response, url *url.URL) {
	bytes, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		log.Printf("Errored on %s:\n ", url.String())
		log.Println(string(bytes))
		return
	}

	respJSON := make(map[string]interface{})
	err := json.Unmarshal(bytes, &respJSON)
	if err == nil {
		log.Printf("Got a response!: %v", respJSON)
	}
}

func main() {
	flag.Parse()
	if *password == "" || *filename == "" {
		fmt.Println("USAGE: ")
		flag.PrintDefaults()
		os.Exit(1)
	}

	log.Printf("Running with password = %s", *password)

	if rateLimited() {
		log.Fatal("Ratelimited, wait for some time.")
	}

	output, err := ioutil.ReadFile(*filename)
	if err != nil {
		log.Fatal(err)
	}
	words := strings.Split(string(output), "\n")

	for _, appStr := range words {
		appID, err := strconv.Atoi(appStr)
		if err != nil {
			log.Fatal(err)
		}
		req := createRequest(*password, appID)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Fatal(err)
		}
		checkResponse(resp, req.URL)
	}

}
