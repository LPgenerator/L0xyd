package helpers

import (
	"strings"
	"net/http"
	"encoding/base64"
	log "github.com/Sirupsen/logrus"
)

type handler func(w http.ResponseWriter, r *http.Request)

func BasicAuth(pass handler) handler {
	return func(w http.ResponseWriter, r *http.Request) {
		if (r.Header["Authorization"] == nil) {
			http.Error(w, "Authorization Required", http.StatusUnauthorized)
			return
		}

		auth := strings.SplitN(r.Header["Authorization"][0], " ", 2)
		if len(auth) != 2 || auth[0] != "Basic" {
			http.Error(w, "bad syntax", http.StatusBadRequest)
			return
		}

		payload, _ := base64.StdEncoding.DecodeString(auth[1])
		pair := strings.SplitN(string(payload), ":", 2)
		if len(pair) != 2 || !Validate(pair[0], pair[1]) {
			http.Error(w, "Authorization Required", http.StatusUnauthorized)
			return
		}
		pass(w, r)
	}
}

func Validate(username, password string) bool {
	if username == "lb" && password == "7eNQ4iWLgDw4Q6w" {
		return true
	}
	return false
}

func GetOnly(h handler) handler {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			h(w, r)
			return
		}
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
	}
}

func PostOnly(h handler) handler {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			h(w, r)
			return
		}
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
	}
}

func LogRequests(h handler) handler {
	return func(w http.ResponseWriter, r *http.Request) {
		h(w, r)
		log.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL)
	}
}
