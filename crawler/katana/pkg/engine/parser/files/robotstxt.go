package files

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/projectdiscovery/retryablehttp-go"
	errorutil "github.com/projectdiscovery/utils/errors"
	"github.com/yhy0/Jie/crawler/katana/pkg/navigation"
	"github.com/yhy0/Jie/crawler/katana/pkg/utils"
)

type robotsTxtCrawler struct {
	httpclient *retryablehttp.Client
}

// Visit visits the provided URL with file crawlers
func (r *robotsTxtCrawler) Visit(URL string, callback func(navigation.Request)) error {
	URL = strings.TrimSuffix(URL, "/")
	requestURL := fmt.Sprintf("%s/robots.txt", URL)
	req, err := retryablehttp.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return errorutil.NewWithTag("robotscrawler", "could not create request").Wrap(err)
	}
	req.Header.Set("User-Agent", utils.WebUserAgent())

	resp, err := r.httpclient.Do(req)
	if err != nil {
		return errorutil.NewWithTag("robotscrawler", "could not do request").Wrap(err)
	}
	defer resp.Body.Close()

	r.parseReader(resp.Body, resp, callback)
	return nil
}

func (r *robotsTxtCrawler) parseReader(reader io.Reader, resp *http.Response, callback func(navigation.Request)) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		text := scanner.Text()
		splitted := strings.SplitN(text, ": ", 2)
		if len(splitted) < 2 {
			continue
		}
		directive := strings.ToLower(splitted[0])
		if strings.HasPrefix(directive, "allow") || strings.EqualFold(directive, "disallow") {
			callback(navigation.NewNavigationRequestURLFromResponse(strings.Trim(splitted[1], " "), resp.Request.URL.String(), "file", "robotstxt", navigation.Response{
				Depth: 2,
				Resp:  resp,
			}))
		}
	}
}
