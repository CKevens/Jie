package hybrid

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sync/atomic"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/launcher/flags"
	"github.com/projectdiscovery/gologger"
	errorutil "github.com/projectdiscovery/utils/errors"
	mapsutil "github.com/projectdiscovery/utils/maps"
	stringsutil "github.com/projectdiscovery/utils/strings"
	"github.com/remeh/sizedwaitgroup"
	ps "github.com/shirou/gopsutil/v3/process"
	"github.com/yhy0/Jie/crawler/katana/pkg/engine/common"
	"github.com/yhy0/Jie/crawler/katana/pkg/engine/parser"
	"github.com/yhy0/Jie/crawler/katana/pkg/engine/parser/files"
	"github.com/yhy0/Jie/crawler/katana/pkg/navigation"
	"github.com/yhy0/Jie/crawler/katana/pkg/output"
	"github.com/yhy0/Jie/crawler/katana/pkg/types"
	"github.com/yhy0/Jie/crawler/katana/pkg/utils"
	"github.com/yhy0/Jie/crawler/katana/pkg/utils/queue"
	"go.uber.org/multierr"
)

// Crawler is a standard crawler instance
type Crawler struct {
	headers      map[string]string
	options      *types.CrawlerOptions
	browser      *rod.Browser
	knownFiles   *files.KnownFiles
	previousPIDs map[int32]struct{} // track already running PIDs
	tempDir      string
}

// New returns a new standard crawler instance
func New(options *types.CrawlerOptions) (*Crawler, error) {
	var dataStore string
	var err error
	if options.Options.ChromeDataDir != "" {
		dataStore = options.Options.ChromeDataDir
	} else {
		dataStore, err = os.MkdirTemp("", "katana-*")
		if err != nil {
			return nil, errorutil.NewWithTag("hybrid", "could not create temporary directory").Wrap(err)
		}
	}

	previousPIDs := findChromeProcesses()

	// todo yhy
	chromeLauncher := launcher.New().
		Leakless(false).
		Set("disable-gpu", "true").
		Set("ignore-certificate-errors", "true").
		Set("ignore-certificate-errors", "1").
		Set("disable-crash-reporter", "true").
		//Set("disable-notifications", "true"). // todo 如果禁用通知，会导致 https://bot.sannysoft.com/ 不是全绿，不知道有什么影响，先注释
		Set("hide-scrollbars", "true").
		Set("window-size", fmt.Sprintf("%d,%d", 1080, 1920)).
		Set("mute-audio", "true").
		Set("disable-images", "true").
		Set("disable-web-security", "true").
		Set("disable-xss-auditor", "true").
		Set("disable-setuid-sandbox", "true").
		Set("allow-running-insecure-content", "true").
		// todo 这个 3d 图形 禁用的话，这个网站 https://bot.sannysoft.com/ webgl 检查是红的(正常浏览器是全绿的)，
		// todo katana 是没有禁用的，其他爬虫(crawlergo)是禁用的，也不知道会不会导致有些检测不通过，先注释
		// todo crawlergo 是通过 https://intoli.com/blog/not-possible-to-block-chrome-headless/chrome-headless-test.html 检查的
		//Set("disable-webgl", "true").
		Set("disable-popup-blocking", "、true").
		//Delete("use-mock-keychain").
		UserDataDir(dataStore)

	if options.Options.UseInstalledChrome {
		if chromePath, hasChrome := launcher.LookPath(); hasChrome {
			chromeLauncher.Bin(chromePath)
		} else {
			return nil, errorutil.NewWithTag("hybrid", "the chrome browser is not installed").WithLevel(errorutil.Fatal)
		}
	}
	if options.Options.SystemChromePath != "" {
		chromeLauncher.Bin(options.Options.SystemChromePath)
	}

	if options.Options.ShowBrowser {
		chromeLauncher = chromeLauncher.Headless(false)
	} else {
		chromeLauncher = chromeLauncher.Headless(true)
	}

	if options.Options.HeadlessNoSandbox {
		chromeLauncher.Set("no-sandbox", "true")
	}

	if options.Options.Proxy != "" && options.Options.Headless {
		proxyURL, err := url.Parse(options.Options.Proxy)
		if err != nil {
			return nil, err
		}
		chromeLauncher.Set("proxy-server", proxyURL.String())
	}

	for k, v := range options.Options.ParseHeadlessOptionalArguments() {
		chromeLauncher.Set(flags.Flag(k), v)
	}

	launcherURL, err := chromeLauncher.Launch()
	if err != nil {
		return nil, err
	}

	browser := rod.New().ControlURL(launcherURL)
	if browserErr := browser.Connect(); browserErr != nil {
		return nil, browserErr
	}

	crawler := &Crawler{
		headers:      options.Options.ParseCustomHeaders(),
		options:      options,
		browser:      browser,
		previousPIDs: previousPIDs,
		tempDir:      dataStore,
	}
	if options.Options.KnownFiles != "" {
		httpclient, _, err := common.BuildClient(options.Dialer, options.Options, nil)
		if err != nil {
			return nil, errorutil.NewWithTag("hybrid", "could not create http client").Wrap(err)
		}
		crawler.knownFiles = files.New(httpclient, options.Options.KnownFiles)
	}
	return crawler, nil
}

// Close closes the crawler process
func (c *Crawler) Close() error {
	if err := c.browser.Close(); err != nil {
		return err
	}
	if c.options.Options.ChromeDataDir == "" {
		if err := os.RemoveAll(c.tempDir); err != nil {
			return err
		}
	}
	return c.killChromeProcesses()
}

// Crawl crawls a URL with the specified options
func (c *Crawler) Crawl(rootURL string) error {
	ctx, cancel := context.WithCancel(context.Background())
	if c.options.Options.CrawlDuration > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(c.options.Options.CrawlDuration)*time.Second)
	}
	defer cancel()

	parsed, err := url.Parse(rootURL)
	if err != nil {
		return errorutil.NewWithTag("hybrid", "could not parse root URL").Wrap(err)
	}
	hostname := parsed.Hostname()

	queue := queue.New(c.options.Options.Strategy)
	queue.Push(navigation.Request{Method: http.MethodGet, URL: rootURL, Depth: 0}, 0)
	parseResponseCallback := c.makeParseResponseCallback(queue)

	if c.knownFiles != nil {
		if err := c.knownFiles.Request(rootURL, func(nr navigation.Request) {
			parseResponseCallback(nr)
		}); err != nil {
			gologger.Warning().Msgf("Could not parse known files for %s: %s\n", rootURL, err)
		}
	}

	httpclient, _, err := common.BuildClient(c.options.Dialer, c.options.Options, func(resp *http.Response, depth int) {
		body, _ := io.ReadAll(resp.Body)
		reader, _ := goquery.NewDocumentFromReader(bytes.NewReader(body))
		navigationResponse := navigation.Response{
			Depth:        depth + 1,
			Options:      c.options,
			RootHostname: hostname,
			Resp:         resp,
			Body:         body,
			Reader:       reader,
			Technologies: mapsutil.GetKeys(c.options.Wappalyzer.Fingerprint(resp.Header, body)),
		}

		parser.ParseResponse(navigationResponse, parseResponseCallback)
	})
	if err != nil {
		return errorutil.NewWithTag("hybrid", "could not create http client").Wrap(err)
	}

	// create a new browser instance (default to incognito mode)
	var newBrowser *rod.Browser
	if c.options.Options.HeadlessNoIncognito {
		if err := c.browser.Connect(); err != nil {
			return err
		}
		newBrowser = c.browser
	} else {
		var err error
		newBrowser, err = c.browser.Incognito()
		if err != nil {
			return err
		}
	}

	wg := sizedwaitgroup.New(c.options.Options.Concurrency)
	running := int32(0)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		// Quit the crawling for zero items or context timeout
		if !(atomic.LoadInt32(&running) > 0) && (queue.Len() == 0) {
			break
		}
		item := queue.Pop()
		req, ok := item.(navigation.Request)
		if !ok {
			continue
		}
		if !utils.IsURL(req.URL) {
			continue
		}
		wg.Add()
		atomic.AddInt32(&running, 1)

		go func() {
			defer wg.Done()
			defer atomic.AddInt32(&running, -1)

			c.options.RateLimit.Take()

			// Delay if the user has asked for it
			if c.options.Options.Delay > 0 {
				time.Sleep(time.Duration(c.options.Options.Delay) * time.Second)
			}
			resp, err := c.navigateRequest(ctx, httpclient, queue, parseResponseCallback, newBrowser, req, hostname)
			if err != nil {
				gologger.Warning().Msgf("Could not request seed URL %s: %s\n", req.URL, err)

				outputError := &output.Error{
					Timestamp: time.Now(),
					Endpoint:  req.RequestURL(),
					Source:    req.Source,
					Error:     err.Error(),
				}
				_ = c.options.OutputWriter.WriteErr(outputError)

				return
			}
			if resp == nil || resp.Resp == nil && resp.Reader == nil {
				return
			}
			// process the dom-rendered response
			parser.ParseResponse(*resp, parseResponseCallback)
		}()
	}
	wg.Wait()

	return nil
}

// makeParseResponseCallback returns a parse response function callback
func (c *Crawler) makeParseResponseCallback(queue *queue.VarietyQueue) func(nr navigation.Request) {
	return func(nr navigation.Request) {
		if nr.URL == "" || !utils.IsURL(nr.URL) {
			return
		}
		parsed, err := url.Parse(nr.URL)
		if err != nil {
			return
		}
		// Ignore blank URL items and only work on unique items
		if !c.options.UniqueFilter.UniqueURL(nr.RequestURL()) && len(nr.CustomFields) == 0 {
			return
		}
		// - URLs stuck in a loop
		if c.options.UniqueFilter.IsCycle(nr.RequestURL()) {
			return
		}

		// Write the found result to output
		result := &output.Result{
			Timestamp:          time.Now(),
			Body:               nr.Body,
			URL:                nr.URL,
			Headers:            nr.Headers,
			Resp:               nr.Resp,
			Source:             nr.Source,
			Tag:                nr.Tag,
			Attribute:          nr.Attribute,
			CustomFields:       nr.CustomFields,
			SourceTechnologies: nr.SourceTechnologies,
		}
		result.Method = nr.Method
		scopeValidated, err := c.options.ScopeManager.Validate(parsed, nr.RootHostname)
		if err != nil {
			return
		}
		if scopeValidated || c.options.Options.DisplayOutScope {
			// todo yhy 获取结果
			c.options.Options.WriteCallback.Write(result)
			_ = c.options.OutputWriter.Write(result, nil)
		}
		if c.options.Options.OnResult != nil {
			c.options.Options.OnResult(*result)
		}
		// Do not add to crawl queue if max items are reached
		if nr.Depth >= c.options.Options.MaxDepth || !scopeValidated {
			return
		}
		queue.Push(nr, nr.Depth)
	}
}

// killChromeProcesses any and all new chrome processes started after
// headless process launch.
func (c *Crawler) killChromeProcesses() error {
	var errs []error
	processes, _ := ps.Processes()

	for _, process := range processes {
		// skip non-chrome processes
		if !isChromeProcess(process) {
			continue
		}

		// skip chrome processes that were already running
		if _, ok := c.previousPIDs[process.Pid]; ok {
			continue
		}

		if err := process.Kill(); err != nil {
			errs = append(errs, err)
		}
	}

	return multierr.Combine(errs...)
}

// findChromeProcesses finds chrome process running on host
func findChromeProcesses() map[int32]struct{} {
	processes, _ := ps.Processes()
	list := make(map[int32]struct{})
	for _, process := range processes {
		if isChromeProcess(process) {
			list[process.Pid] = struct{}{}
			if ppid, err := process.Ppid(); err == nil {
				list[ppid] = struct{}{}
			}
		}
	}
	return list
}

// isChromeProcess checks if a process is chrome/chromium
func isChromeProcess(process *ps.Process) bool {
	name, _ := process.Name()
	executable, _ := process.Exe()
	return stringsutil.ContainsAny(name, "chrome", "chromium") || stringsutil.ContainsAny(executable, "chrome", "chromium")
}
