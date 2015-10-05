package statistics

import (
	"net/http"
	"github.com/thoas/stats"
	"github.com/mailgun/oxy/utils"
)

type Statistics struct {
	next       http.Handler
	stats      *stats.Stats
}

func New(next http.Handler, sts *stats.Stats) (*Statistics, error) {
	strm := &Statistics{
		next: next,
		stats: sts,
	}
	return strm, nil
}

func (s *Statistics) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-A", "LP-LB")
	beginning, _ := s.stats.Begin(w)
	pw := &utils.ProxyWriter{W: w}
	s.next.ServeHTTP(pw, r)
	s.stats.EndWithStatus(beginning, pw.Code)
}
