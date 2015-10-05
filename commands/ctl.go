package commands

import (
	"fmt"
	//"bytes"
	"strings"
	"net/url"
	"net/http"
	"io/ioutil"
	"encoding/json"

	"github.com/codegangsta/cli"
	log "github.com/Sirupsen/logrus"
	"git.lpgenerator.ru/sys/lpg-load-balancer/common"
)

type CtlCommand struct {
	configOptions

	LbAction  string `short:"a" long:"action" description:"add/rm/list/stats"`
	LbBackend string `short:"b" long:"backend" description:"127.0.0.1:8081"`
}

func (c *CtlCommand) doRequest(method string, path string, backend string) string {
	uri := fmt.Sprintf("http://%s%s", c.config.ApiAddress, path)

	data := url.Values{}
	if backend != "" {
		data.Set("url", backend)
		data.Set("weight", "1")
	}

	r, err := http.NewRequest(method, uri, strings.NewReader(data.Encode()))
	r.Header.Add("Authorization", "Basic bGI6N2VOUTRpV0xnRHc0UTZ3")
	if backend != "" {
		r.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	}
	if err == nil {
		re, err := http.DefaultClient.Do(r)
		if err == nil {
			if re.Body != nil {
				defer re.Body.Close()
			}
			bodyText, err := ioutil.ReadAll(re.Body)
			if err == nil {
				var json_data interface{}
				err := json.Unmarshal([]byte(bodyText), &json_data)
				if err == nil {
					data, _ := json.MarshalIndent(json_data, "", "  ")
					return string(data)
				}
			}
		}
	}
	return "ERROR"
}

func (c *CtlCommand) Execute(context *cli.Context) {
	err := c.loadConfig()
	if err != nil {
		log.Fatalln(err)
		return
	}

	if c.LbAction == "list" {
		fmt.Println(c.doRequest("GET", "/", ""))
	} else if c.LbAction == "stats" {
		fmt.Println(c.doRequest("GET", "/stats", ""))
	} else if c.LbAction == "add" {
		fmt.Println(c.doRequest("PUT", "/", c.LbBackend))
	} else if c.LbAction == "delete" {
		fmt.Println(c.doRequest("DELETE", "/" + c.LbBackend, ""))
	} else {
		log.Println("Unknown")
	}
}

func init() {
	common.RegisterCommand2("ctl",  "Simple control command", &CtlCommand{})
}
