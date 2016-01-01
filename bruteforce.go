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
	"sync"
	"sync/atomic"
	"time"

	"github.com/cheggaaa/pb"
	_ "github.com/olekukonko/ts"
)

var (
	appFilename = flag.String("appfile", "", "App list file")
	pwdFilename = flag.String("pwdfile", "", "Password list file")
	parallel    = flag.Int("parallel", 10, "number of GETs to send at once")
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

func checkResponse(resp *http.Response, req *http.Request) {
	bytes, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		log.Printf("Errored on %s:\n ", req.URL.String())
		log.Println(string(bytes))
		return
	}

	respJSON := make(map[string]interface{})
	err := json.Unmarshal(bytes, &respJSON)
	if err == nil {
		log.Printf("On %s with Referer %s", req.URL.String(), req.Header["Referer"][0])
		log.Printf("Got a response!: %v", respJSON)
	}
}

func main() {
	flag.Parse()
	if *appFilename == "" || *pwdFilename == "" {
		fmt.Println("USAGE: ")
		flag.PrintDefaults()
		os.Exit(1)
	}

	if rateLimited() {
		log.Fatal("Ratelimited, wait for some time.")
	}

	appOutput, err := ioutil.ReadFile(*appFilename)
	if err != nil {
		log.Fatal(err)
	}
	apps := strings.Split(string(appOutput), "\n")

	pwdOutput, err := ioutil.ReadFile(*pwdFilename)
	if err != nil {
		log.Fatal(err)
	}
	pwds := strings.Split(string(pwdOutput), "\n")

	log.Printf("Sending %d GETs at once.", *parallel)
	bar := pb.New(len(apps))
	bar.SetRefreshRate(time.Second)
	bar.ShowSpeed = true
	bar.SetWidth(100)
	bar.SetMaxWidth(100)
	bar.SetUnits(pb.Units(len(apps) * len(pwds)))
	bar.Start()

	wait := new(sync.WaitGroup)
	reqCount := int32(0)
	for _, appStr := range apps {
		for _, password := range pwds {
			if atomic.LoadInt32(&reqCount) == int32(*parallel) {
				wait.Wait()
				atomic.StoreInt32(&reqCount, 0)
				bar.Add(*parallel)
			}

			wait.Add(1)
			atomic.AddInt32(&reqCount, 1)
			go func(password, appStr string) {
				appID, err := strconv.Atoi(appStr)
				if err != nil {
					log.Fatal(err)
				}
				req := createRequest(password, appID)
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					log.Fatal(err)
				}
				checkResponse(resp, req)
				wait.Done()
			}(password, appStr)

		}
	}

	wait.Wait()
	bar.Set(len(apps) * len(pwds))
}
