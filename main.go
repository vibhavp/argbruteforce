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
	serverURL   = flag.String("url", "", "Server URL")
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

func checkResponse(resp *http.Response, req *http.Request, appID int) (bool, bool) {
	bytes, _ := ioutil.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		if resp.StatusCode == 408 {
			return true, false
		}
		log.Printf("Errored on %s:\n ", req.URL.String())
		log.Println(string(bytes))
		return false, false
	}

	respJSON := make(map[string]interface{})
	err := json.Unmarshal(bytes, &respJSON)
	if err == nil {
		fmt.Println("")
		log.Printf("On %s with Referer %s", req.URL.String(), req.Header["Referer"][0])
		log.Printf("Got a response!: %v", respJSON)
		if *serverURL == "" {
			return false, true
		}

		u, err := url.Parse(*serverURL)
		if err != nil {
			log.Println(err)
			return false, true
		}
		query := u.Query()
		query.Set("pwd", req.URL.Query().Get("key"))
		query.Set("appid", strconv.Itoa(appID))
		u.RawQuery = query.Encode()

		_, err = http.DefaultClient.Get(u.String())
		if err != nil {
			log.Println(err)
		}
		return false, true
	}
	return false, false
}

func getPasswords(URL string) []string {
	u, err := url.Parse(URL)
	if err != nil {
		log.Println(err)
	}

	u.Path = "get"
	resp, err := http.DefaultClient.Get(u.String())
	if err != nil {
		log.Fatal(err)
	}
	var pwds []string
	bytes, _ := ioutil.ReadAll(resp.Body)
	json.Unmarshal(bytes, &pwds)
	log.Println(pwds)

	return pwds
}

func main() {
	// go func() {
	// 	log.Println(http.ListenAndServe("localhost:6060", nil))
	// }()

	log.SetFlags(log.Lshortfile)
	flag.Parse()
	if *mode == "server" {
		pwdOutput, err := ioutil.ReadFile(*pwdFilename)
		if err != nil {
			log.Fatal(err)
		}
		passwords = strings.Split(string(pwdOutput), "\n")
		log.Printf("Read %d passwords", len(passwords))

		startServer()
		os.Exit(0)
	} else if *mode == "server+client" {
		go startServer()
	}

	if *appFilename == "" || (*pwdFilename == "" && *mode == "client" && *serverURL == "") {
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

	if *mode == "client" && *serverURL != "" {
		passwords = getPasswords(*serverURL)
	} else {
		pwdOutput, err := ioutil.ReadFile(*pwdFilename)
		if err != nil {
			log.Fatal(err)
		}
		passwords = strings.Split(string(pwdOutput), "\n")
		log.Printf("Read %d passwords", len(passwords))
	}

	log.Printf("Sending %d GETs at once.", *parallel)
	bar := pb.New(len(apps))
	bar.SetRefreshRate(time.Second)
	bar.ShowSpeed = true
	bar.SetWidth(100)
	bar.SetMaxWidth(100)
	bar.SetUnits(pb.Units(len(apps) * len(passwords)))
	bar.Start()

	invalidPwd := make(chan string, *parallel)
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

	go func() {
		donePwd := make(map[string]int)
		for {
			pwd := <-invalidPwd
			wait.Add(1)
			donePwd[pwd]++
			if donePwd[pwd] == len(apps) && *serverURL != "" {
				u, err := url.Parse(*serverURL)
				if err != nil {
					log.Println(err)
				}
				u.Path = "invalid"
				values := u.Query()
				values.Set("pwd", pwd)
				u.RawQuery = values.Encode()

				_, err = http.DefaultClient.Get(u.String())
				if err != nil {
					log.Println(err)
				}
			}

			wait.Done()

		}
	}()

	for _, password := range passwords {
		for _, appStr := range apps {
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

				timeout, valid := checkResponse(resp, req, appID)
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
					timeout, valid = checkResponse(resp, req, appID)
				}

				if !valid {
					invalidPwd <- password
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
