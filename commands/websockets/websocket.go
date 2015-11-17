package websockets

import (
	"net/http"
	"net/url"
	"strings"
	//"time"

	log "github.com/Sirupsen/logrus"
	//"github.com/mailgun/oxy/roundrobin"
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
		//start := time.Now().UTC()
		ws_url, _ := url.Parse(req.URL.String()+req.RequestURI)
		log.Debugf("Websocket forward to %s", ws_url)
		NewProxy(ws_url).ServeHTTP(w, req)

		/*
		if req.TLS != nil {
			log.Debugf("Round trip: %v, duration: %v tls:version: %x, tls:resume:%t, tls:csuite:%x, tls:server:%v",
				req.URL, time.Now().UTC().Sub(start),
				req.TLS.Version,
				req.TLS.DidResume,
				req.TLS.CipherSuite,
				req.TLS.ServerName)
		} else {
			log.Debugf("Round trip: %v, duration: %v", req.URL, time.Now().UTC().Sub(start))
		}
		*/
		return
	}
	ws.next.ServeHTTP(w, req)
}
