package mirror

import (
	"strings"
	"net/url"
	"net/http"
	"io/ioutil"

	"github.com/mailgun/oxy/utils"
)

type Mirroring struct {
	next        http.Handler
	mirrors     []string
	rewriter    ReqRewriter
	methods     map[string]bool
}

type ReqRewriter interface {
	Rewrite(r *http.Request)
}

func New(next http.Handler, methods string) (*Mirroring, error) {
	strm := &Mirroring{
		next:    next,
		methods: make(map[string]bool),
	}
	for _, m := range strings.Split(methods, "|") {
		strm.methods[m] = true
	}
	return strm, nil
}

func (m *Mirroring) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	pw := &utils.ProxyWriter{W: w}
	if m.methods[r.Method] {
		for _, mirror := range m.mirrors {
			m.mirrorRequest(mirror, w, r)
		}
	}
	m.next.ServeHTTP(pw, r)
}

func (m *Mirroring) mirrorRequest(backend string, w http.ResponseWriter, req *http.Request) {
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


func (m *Mirroring) Add(mirror string) {
	m.mirrors = append(m.mirrors, mirror)
}

func (m *Mirroring) Del(mirror string) {
    for i, url := range m.mirrors {
		if url == mirror {
			m.mirrors = append(m.mirrors[:i], m.mirrors[i+1:]...)
			break
		}
    }
}
