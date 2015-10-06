package headers

import (
	"net/http"
	"github.com/LPgenerator/L0xyd/common"
)

type XHeader struct{
	next            http.Handler
	config          *common.Config
}

func New(next http.Handler, config *common.Config) (*XHeader, error) {
	strm := &XHeader{
		next:       next,
		config:     config,
	}
	return strm, nil
}

func (x *XHeader) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(x.config.LbXHeaderKey, x.config.LbXHeaderVal)
	x.next.ServeHTTP(w, r)
}
