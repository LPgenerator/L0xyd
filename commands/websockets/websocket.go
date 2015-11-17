package websockets

import (
	"net/http"
	"net/url"
	"strings"

	log "github.com/Sirupsen/logrus"
)

type WebSockets struct {
	next            http.Handler
}


func New(next http.Handler) (*WebSockets, error) {
	strm := &WebSockets{
		next:       next,
	}
	return strm, nil
}

func (ws *WebSockets) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if strings.Join(req.Header["Upgrade"], "") == "websocket" {
		ws_url, _ := url.Parse(req.URL.String()+req.RequestURI)
		log.Debugf("Websocket forward to %s", ws_url)
		NewProxy(ws_url).ServeHTTP(w, req)
		return
	}
	ws.next.ServeHTTP(w, req)
}
