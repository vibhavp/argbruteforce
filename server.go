//+build !windows !darwin !arm

package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/tylerb/graceful"
)

var (
	db            *sql.DB
	lastID        uint64
	lastInvalidID uint64

	port = flag.Int("port", 8080, "Port to listen for queries on")
)

func startServer() {
	var err error

	db, err = sql.Open("sqlite3", "./valid.db")
	if err != nil {
		log.Fatal(err)
	}
	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec("CREATE TABLE valid (id integer not null primary key, app_id integer, password varchar)")
	if err != nil && err.Error() != "table valid already exists" {
		log.Fatal(err)
	}

	_, err = db.Exec("CREATE TABLE invalid (id integer not null primary key, password varchar)")
	if err != nil && err.Error() != "table invalid already exists" {
		log.Fatal(err)
	}

	rows, _ := db.Query("SELECT id FROM valid")
	for rows.Next() {
		rows.Scan(&lastID)
	}
	lastID++
	log.Printf("Last ID: %d", lastID)

	rows, _ = db.Query("SELECT id FROM invalid")
	for rows.Next() {
		rows.Scan(&lastInvalidID)
	}
	lastInvalidID++
	log.Printf("Last Invalid ID: %d", lastInvalidID)

	http.HandleFunc("/", newEntry)
	http.HandleFunc("/invalid", invalidEntry)
	http.HandleFunc("/get", get)

	log.Printf("Listening on %d", *port)
	graceful.Run(fmt.Sprintf(":%d", *port), 1*time.Second, http.DefaultServeMux)
}

func get(w http.ResponseWriter, r *http.Request) {
	var final []string
	for _, pwd := range passwords {
		rows, err := db.Query("SELECT id FROM invalid WHERE password = ?", pwd)
		if err != nil && rows.Next() {
			//invalid password
			continue
		}
		final = append(final, pwd)
	}

	bytes, _ := json.Marshal(final)
	fmt.Fprintf(w, string(bytes))
	return
}

func newEntry(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	pwd := query.Get("pwd")
	appIDStr := query.Get("appid")

	if appIDStr == "" {
		http.Error(w, "empty appid", 400)
		return
	}
	if pwd == "" {
		http.Error(w, "invalid password", 400)
		return
	}

	appID, err := strconv.Atoi(appIDStr)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	_, err = db.Exec("insert into valid(id, app_id, password) values(?, ?, ?)", lastID, appID, pwd)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	atomic.AddUint64(&lastID, 1)
	resp := &http.Response{Status: "200 OK", StatusCode: http.StatusOK}
	resp.Write(w)
	log.Printf("New Entry - AppID: %d, Password: %s", appID, pwd)
	passwordMu.Lock()
	for i, passwd := range passwords {
		if passwd == pwd {
			passwords[i] = passwords[len(passwords)-1]
			passwords = passwords[:len(passwords)-1]
		}
	}
	passwordMu.Unlock()
}

func invalidEntry(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	pwd := query.Get("pwd")

	if pwd == "" {
		http.Error(w, "invalid password", 400)
		return
	}

	_, err := db.Exec("insert into invalid(id, password) values(?,?)", lastInvalidID, pwd)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	atomic.AddUint64(&lastInvalidID, 1)
	resp := &http.Response{Status: "200 OK", StatusCode: http.StatusOK}
	resp.Write(w)
	log.Printf("New Invalid Entry - Password: %s", pwd)
}
