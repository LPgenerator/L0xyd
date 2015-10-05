package mirror

import (
	//"fmt"
	"net/url"
	"net/http"
	"io/ioutil"

	"github.com/mailgun/oxy/utils"
)

type Mirror struct {
	next       http.Handler
	mirrors    []string
	rewriter    ReqRewriter
}

type ReqRewriter interface {
	Rewrite(r *http.Request)
}

func New(next http.Handler) (*Mirror, error) {
	strm := &Mirror{
		next: next,
	}
	return strm, nil
}

func (m *Mirror) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// fmt.Println(m.mirrors)
	pw := &utils.ProxyWriter{W: w}
	if r.Method == "GET" || r.Method == "HEAD" {
		for _, mirror := range m.mirrors {
			m.mirrorRequest(mirror, w, r)
		}
	}

	m.next.ServeHTTP(pw, r)
}

func (m *Mirror) mirrorRequest(backend string, w http.ResponseWriter, req *http.Request) {
	outReq := new(http.Request)
	*outReq = *req

	u, err := url.Parse(backend)
	outReq.URL = utils.CopyURL(u)
	outReq.URL.Opaque = req.RequestURI
	outReq.URL.RawQuery = ""
	outReq.Proto = "HTTP/1.1"
	outReq.ProtoMajor = 1
	outReq.ProtoMinor = 1
	outReq.Close = false

	outReq.Header = make(http.Header)
	utils.CopyHeaders(outReq.Header, req.Header)
	outReq.RequestURI = ""

	client := &http.Client{}
	response, err := client.Do(outReq)

	if err == nil {
		ioutil.ReadAll(response.Body)
	}
}


func (m *Mirror) Add(mirror string) {
	m.mirrors = append(m.mirrors, mirror)
}

func (m *Mirror) Del(mirror string) {
    for i, url := range m.mirrors {
		if url == mirror {
			m.mirrors = append(m.mirrors[:i], m.mirrors[i+1:]...)
			break
		}
    }
}
