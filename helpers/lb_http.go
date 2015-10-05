package helpers

import (
	"io"
	"net/http"

	"github.com/mailgun/oxy/utils"
	log "github.com/Sirupsen/logrus"
)

type handler func(w http.ResponseWriter, r *http.Request)

func BasicAuth(pass handler, username string, password string) handler {
	return func(w http.ResponseWriter, r *http.Request) {
		auth, err := utils.ParseAuthHeader(r.Header.Get("Authorization"))
		if err != nil || username != auth.Username || password != auth.Password {
			w.WriteHeader(http.StatusForbidden)
			io.WriteString(w, "Forbidden")
			return
		}
		pass(w, r)
	}
}

func LogRequests(h handler) handler {
	return func(w http.ResponseWriter, r *http.Request) {
		h(w, r)
		log.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL)
	}
}
