package common

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"time"

	"github.com/projectdiscovery/fastdialer/fastdialer"
	"github.com/projectdiscovery/retryablehttp-go"
	errorutil "github.com/projectdiscovery/utils/errors"
	"github.com/yhy0/Jie/crawler/katana/pkg/navigation"
	"github.com/yhy0/Jie/crawler/katana/pkg/types"
)

// BuildClient builds a http client based on a profile
func BuildClient(dialer *fastdialer.Dialer, options *types.Options, redirectCallback func(resp *http.Response, depth int)) (*retryablehttp.Client, *fastdialer.Dialer, error) {
	var err error
	var proxyURL *url.URL
	if options.Proxy != "" {
		proxyURL, err = url.Parse(options.Proxy)
	}
	if err != nil {
		return nil, nil, err
	}

	// Single Host
	retryablehttpOptions := retryablehttp.DefaultOptionsSingle
	retryablehttpOptions.RetryMax = options.Retries
	transport := &http.Transport{
		DialContext:         dialer.Dial,
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		MaxConnsPerHost:     100,
		TLSClientConfig: &tls.Config{
			Renegotiation:      tls.RenegotiateOnceAsClient,
			InsecureSkipVerify: true,
		},
		DisableKeepAlives: false,
	}

	// Attempts to overwrite the dial function with the socks proxied version
	if proxyURL != nil {
		transport.Proxy = http.ProxyURL(proxyURL)
	}

	client := retryablehttp.NewWithHTTPClient(&http.Client{
		Transport: transport,
		Timeout:   time.Duration(options.Timeout) * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) == 10 {
				return errorutil.New("stopped after 10 redirects")
			}
			depth, ok := req.Context().Value(navigation.Depth{}).(int)
			if !ok {
				depth = 2
			}
			if redirectCallback != nil {
				redirectCallback(req.Response, depth)
			}
			return nil
		},
	}, retryablehttpOptions)
	client.CheckRetry = retryablehttp.HostSprayRetryPolicy()
	return client, dialer, nil
}
