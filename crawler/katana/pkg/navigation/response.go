package navigation

import (
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Response is a response generated from crawler navigation
type Response struct {
	Resp         *http.Response    `json:"-"`
	Depth        int               `json:"-"`
	Reader       *goquery.Document `json:"-"`
	StatusCode   int               `json:"status_code,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	Body         string            `json:"body,omitempty"`
	RootHostname string            `json:"-"`
	Technologies []string          `json:"technologies,omitempty"`
}

func (n Response) AbsoluteURL(path string) string {
	if strings.HasPrefix(path, "#") {
		return ""
	}

	absURL, err := n.Resp.Request.URL.Parse(path)
	if err != nil {
		return ""
	}
	absURL.Fragment = ""
	if absURL.Scheme == "//" {
		absURL.Scheme = n.Resp.Request.URL.Scheme
	}
	final := absURL.String()
	return final
}
