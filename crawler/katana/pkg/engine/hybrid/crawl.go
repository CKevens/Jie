package hybrid

import (
	"bytes"
	"context"
	_ "embed"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/retryablehttp-go"
	errorutil "github.com/projectdiscovery/utils/errors"
	mapsutil "github.com/projectdiscovery/utils/maps"
	"github.com/yhy0/Jie/crawler/katana/pkg/engine/parser"
	"github.com/yhy0/Jie/crawler/katana/pkg/navigation"
	"github.com/yhy0/Jie/crawler/katana/pkg/utils/queue"
)

// https://github.com/requireCool/stealth.min.js

//go:embed stealth.min.js
var stealthJS string

func (c *Crawler) navigateRequest(ctx context.Context, httpclient *retryablehttp.Client, queue *queue.VarietyQueue, parseResponseCallback func(nr navigation.Request), browser *rod.Browser, request navigation.Request, rootHostname string) (*navigation.Response, error) {
	depth := request.Depth + 1
	response := &navigation.Response{
		Depth:        depth,
		Options:      c.options,
		RootHostname: rootHostname,
	}

	page, err := browser.Page(proto.TargetCreateTarget{})
	// todo yhy 绕过无头浏览器检测 https://bot.sannysoft.com/
	page.MustEvalOnNewDocument(stealthJS)
	if err != nil {
		return nil, errorutil.NewWithTag("hybrid", "could not create target").Wrap(err)
	}
	defer page.Close()

	pageRouter := NewHijack(page)
	pageRouter.SetPattern(&proto.FetchRequestPattern{
		URLPattern:   "*",
		RequestStage: proto.FetchRequestStageResponse,
	})
	go pageRouter.Start(func(e *proto.FetchRequestPaused) error {
		URL, _ := url.Parse(e.Request.URL)
		body, _ := FetchGetResponseBody(page, e)
		headers := make(map[string][]string)
		for _, h := range e.ResponseHeaders {
			headers[h.Name] = []string{h.Value}
		}
		var statuscode int
		if e.ResponseStatusCode != nil {
			statuscode = *e.ResponseStatusCode
		}
		httpresp := &http.Response{
			StatusCode: statuscode,
			Status:     e.ResponseStatusText,
			Header:     headers,
			Body:       io.NopCloser(bytes.NewReader(body)),
			Request: &http.Request{
				Method: e.Request.Method,
				URL:    URL,
				Body:   io.NopCloser(strings.NewReader(e.Request.PostData)),
			},
		}
		if !c.options.UniqueFilter.UniqueContent(body) {
			return FetchContinueRequest(page, e)
		}

		bodyReader, _ := goquery.NewDocumentFromReader(bytes.NewReader(body))
		technologies := c.options.Wappalyzer.Fingerprint(headers, body)
		resp := navigation.Response{
			Resp:         httpresp,
			Body:         []byte(body),
			Reader:       bodyReader,
			Options:      c.options,
			Depth:        depth,
			RootHostname: rootHostname,
			Technologies: mapsutil.GetKeys(technologies),
		}

		// process the raw response
		parser.ParseResponse(resp, parseResponseCallback)
		return FetchContinueRequest(page, e)
	})() //nolint
	defer func() {
		if err := pageRouter.Stop(); err != nil {
			gologger.Warning().Msgf("%s\n", err)
		}
	}()

	timeout := time.Duration(c.options.Options.Timeout) * time.Second
	page = page.Timeout(timeout)

	// wait the page to be fully loaded and becoming idle
	waitNavigation := page.WaitNavigation(proto.PageLifecycleEventNameFirstMeaningfulPaint)

	if err := page.Navigate(request.URL); err != nil {
		return nil, errorutil.NewWithTag("hybrid", "could not navigate target").Wrap(err)
	}
	waitNavigation()

	// Wait for the window.onload event
	if err := page.WaitLoad(); err != nil {
		gologger.Warning().Msgf("\"%s\" on wait load: %s\n", request.URL, err)
	}

	// wait for idle the network requests
	if err := page.WaitIdle(timeout); err != nil {
		gologger.Warning().Msgf("\"%s\" on wait idle: %s\n", request.URL, err)
	}

	var getDocumentDepth = int(-1)
	getDocument := &proto.DOMGetDocument{Depth: &getDocumentDepth, Pierce: true}
	result, err := getDocument.Call(page)
	if err != nil {
		return nil, errorutil.NewWithTag("hybrid", "could not get dom").Wrap(err)
	}
	var builder strings.Builder
	traverseDOMNode(result.Root, &builder)

	body, err := page.HTML()
	if err != nil {
		return nil, errorutil.NewWithTag("hybrid", "could not get html").Wrap(err)
	}

	parsed, _ := url.Parse(request.URL)
	response.Resp = &http.Response{Header: make(http.Header), Request: &http.Request{URL: parsed}}

	// Create a copy of intrapolated shadow DOM elements and parse them separately
	responseCopy := *response
	responseCopy.Body = []byte(builder.String())
	if !c.options.UniqueFilter.UniqueContent(responseCopy.Body) {
		return &navigation.Response{}, nil
	}

	responseCopy.Reader, _ = goquery.NewDocumentFromReader(bytes.NewReader(responseCopy.Body))
	if responseCopy.Reader != nil {
		parser.ParseResponse(responseCopy, parseResponseCallback)
	}

	response.Body = []byte(body)
	if !c.options.UniqueFilter.UniqueContent(response.Body) {
		return &navigation.Response{}, nil
	}
	response.Reader, err = goquery.NewDocumentFromReader(bytes.NewReader(response.Body))
	if err != nil {
		return nil, errorutil.NewWithTag("hybrid", "could not parse html").Wrap(err)
	}
	return response, nil
}

// traverseDOMNode performs traversal of node completely building a pseudo-HTML
// from it including the Shadow DOM, Pseudo elements and other children.
//
// TODO: Remove this method when we implement human-like browser navigation
// which will anyway use browser APIs to find elements instead of goquery
// where they will have shadow DOM information.
func traverseDOMNode(node *proto.DOMNode, builder *strings.Builder) {
	buildDOMFromNode(node, builder)
	if node.TemplateContent != nil {
		traverseDOMNode(node.TemplateContent, builder)
	}
	if node.ContentDocument != nil {
		traverseDOMNode(node.ContentDocument, builder)
	}
	for _, children := range node.Children {
		traverseDOMNode(children, builder)
	}
	for _, shadow := range node.ShadowRoots {
		traverseDOMNode(shadow, builder)
	}
	for _, pseudo := range node.PseudoElements {
		traverseDOMNode(pseudo, builder)
	}
}

const (
	elementNode = 1
)

var knownElements = map[string]struct{}{
	"a": {}, "applet": {}, "area": {}, "audio": {}, "base": {}, "blockquote": {}, "body": {}, "button": {}, "embed": {}, "form": {}, "frame": {}, "html": {}, "iframe": {}, "img": {}, "import": {}, "input": {}, "isindex": {}, "link": {}, "meta": {}, "object": {}, "script": {}, "svg": {}, "table": {}, "video": {},
}

func buildDOMFromNode(node *proto.DOMNode, builder *strings.Builder) {
	if node.NodeType != elementNode {
		return
	}
	if _, ok := knownElements[node.LocalName]; !ok {
		return
	}
	builder.WriteRune('<')
	builder.WriteString(node.LocalName)
	builder.WriteRune(' ')
	if len(node.Attributes) > 0 {
		for i := 0; i < len(node.Attributes); i = i + 2 {
			builder.WriteString(node.Attributes[i])
			builder.WriteRune('=')
			builder.WriteString("\"")
			builder.WriteString(node.Attributes[i+1])
			builder.WriteString("\"")
			builder.WriteRune(' ')
		}
	}
	builder.WriteRune('>')
	builder.WriteString("</")
	builder.WriteString(node.LocalName)
	builder.WriteRune('>')
}
