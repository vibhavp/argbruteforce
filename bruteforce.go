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
	//_ "net/http/pprof"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cheggaaa/pb"
	_ "github.com/olekukonko/ts"
)

var (
	appFilename = flag.String("appfile", "", "App list file")
	pwdFilename = flag.String("pwdfile", "", "Password list file")
	parallel    = flag.Int("parallel", 10, "number of GETs to send at once")
	mode        = flag.String("runas", "client", `Run as (valid value: "server", "client". Default: "client")`)
	passwords   []string
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

func checkResponse(resp *http.Response, req *http.Request) bool {
	bytes, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		if resp.StatusCode == 408 {
			return true
		}
		log.Printf("Errored on %s:\n ", req.URL.String())
		log.Println(string(bytes))
		return false
	}

	respJSON := make(map[string]interface{})
	err := json.Unmarshal(bytes, &respJSON)
	if err == nil {
		fmt.Println("")
		log.Printf("On %s with Referer %s", req.URL.String(), req.Header["Referer"][0])
		log.Printf("Got a response!: %v", respJSON)
	}
	return false
}

func main() {
	// go func() {
	// 	log.Println(http.ListenAndServe("localhost:6060", nil))
	// }()

	flag.Parse()
	if *mode == "server" {
		startServer()
		os.Exit(0)
	} else if *mode == "server+client" {
		go startServer()
	}

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
	passwords = strings.Split(string(pwdOutput), "\n")

	log.Printf("Sending %d GETs at once.", *parallel)
	bar := pb.New(len(apps))
	bar.SetRefreshRate(time.Second)
	bar.ShowSpeed = true
	bar.SetWidth(100)
	bar.SetMaxWidth(100)
	bar.SetUnits(pb.Units(len(apps) * len(passwords)))
	bar.Start()

	incProgress := make(chan struct{}, *parallel)
	wait := new(sync.WaitGroup)
	timeoutWait := new(sync.WaitGroup)
	reqCount := 0

	go func() {
		for {
			<-incProgress
			bar.Increment()
		}
	}()

	for _, appStr := range apps {
		for _, password := range passwords {
			timeoutWait.Wait()

			if reqCount == *parallel {
				wait.Wait()
				reqCount = 0
			}

			wait.Add(1)
			reqCount++
			go func(password, appStr string) {
				defer wait.Done()
				appID, err := strconv.Atoi(appStr)
				if err != nil {
					if appStr == "\n" {
						return
					}
					log.Fatal(err)
				}

				req := createRequest(password, appID)
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					fmt.Println("")
					//log.Println(err)
					for err != nil {
						resp, err = http.DefaultClient.Do(req)
					}
				}

				timeout := checkResponse(resp, req)
				if timeout {
					timeoutWait.Add(1)
					defer timeoutWait.Done()
				}

				for timeout {
					time.Sleep(1 * time.Minute)
					resp, err := http.DefaultClient.Do(req)
					if err != nil {
						log.Fatal(err)
					}
					timeout = checkResponse(resp, req)
				}

				incProgress <- struct{}{}
			}(password, appStr)

		}
	}

	wait.Wait()
	//bar.Set(len(apps) * len(pwds))
	bar.Finish()
	log.Println("Finished!")
}
