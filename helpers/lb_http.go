package helpers

import (
	"net/http"

	"github.com/mailgun/oxy/utils"
	log "github.com/Sirupsen/logrus"
)

type handler func(w http.ResponseWriter, r *http.Request)

func LogRequests(h handler) handler {
	return func(w http.ResponseWriter, r *http.Request) {
		h(w, r)
		log.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL)
	}
}

type LMiddleware struct {}
type AMiddleware struct {
	Username string
	Password string
}

func LogMiddleware() *LMiddleware {
	return &LMiddleware{}
}

func (l *LMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	next(w, r)
	log.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL)
}


func AuthMiddleware(username string, password string) *AMiddleware {
	return &AMiddleware{
		Username: username,
		Password: password,
	}
}

func (a *AMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	auth, err := utils.ParseAuthHeader(r.Header.Get("Authorization"))
	if err != nil || a.Username != auth.Username || a.Password != auth.Password {
		w.Header().Set("WWW-Authenticate", `Basic realm="Auth required"`)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	next(w, r)
}
