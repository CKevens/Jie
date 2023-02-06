package types

import (
	"context"
	"time"

	"github.com/projectdiscovery/fastdialer/fastdialer"
	"github.com/projectdiscovery/ratelimit"
	errorutil "github.com/projectdiscovery/utils/errors"
	"github.com/yhy0/Jie/crawler/katana/pkg/output"
	"github.com/yhy0/Jie/crawler/katana/pkg/utils/extensions"
	"github.com/yhy0/Jie/crawler/katana/pkg/utils/filters"
	"github.com/yhy0/Jie/crawler/katana/pkg/utils/scope"
	wappalyzer "github.com/yhy0/wappalyzergo"
)

// CrawlerOptions contains helper utilities for the crawler
type CrawlerOptions struct {
	// OutputWriter is the interface for writing output
	OutputWriter output.Writer
	// RateLimit is a mechanism for controlling request rate limit
	RateLimit ratelimit.Limiter
	// Options contains the user specified configuration options
	Options *Options
	// ExtensionsValidator is a validator for file extensions
	ExtensionsValidator *extensions.Validator
	// UniqueFilter is a filter for deduplication of unique items
	UniqueFilter filters.Filter
	// ScopeManager is a manager for validating crawling scope
	ScopeManager *scope.Manager
	// Dialer is instance of the dialer for global crawler
	Dialer *fastdialer.Dialer
	// Wappalyzer instance for technologies detection
	Wappalyzer *wappalyzer.Wappalyze
}

// NewCrawlerOptions creates a new crawler options structure
// from user specified options.
func NewCrawlerOptions(options *Options) (*CrawlerOptions, error) {
	extensionsValidator := extensions.NewValidator(options.ExtensionsMatch, options.ExtensionFilter)

	fastdialerInstance, err := fastdialer.NewDialer(fastdialer.DefaultOptions)
	if err != nil {
		return nil, err
	}
	scopeManager, err := scope.NewManager(options.Scope, options.OutOfScope, options.FieldScope, options.NoScope)
	if err != nil {
		return nil, errorutil.NewWithErr(err).Msgf("could not create scope manager")
	}
	itemFilter, err := filters.NewSimple()
	if err != nil {
		return nil, errorutil.NewWithErr(err).Msgf("could not create filter")
	}

	outputOptions := output.Options{
		Colors:           !options.NoColors,
		JSON:             options.JSON,
		Verbose:          options.Verbose,
		StoreResponse:    options.StoreResponse,
		OutputFile:       options.OutputFile,
		Fields:           options.Fields,
		StoreFields:      options.StoreFields,
		StoreResponseDir: options.StoreResponseDir,
		FieldConfig:      options.FieldConfig,
		ErrorLogFile:     options.ErrorLogFile,
	}
	outputWriter, err := output.New(outputOptions)
	if err != nil {
		return nil, errorutil.NewWithErr(err).Msgf("could not create output writer")
	}

	crawlerOptions := &CrawlerOptions{
		ExtensionsValidator: extensionsValidator,
		ScopeManager:        scopeManager,
		UniqueFilter:        itemFilter,
		Options:             options,
		Dialer:              fastdialerInstance,
		OutputWriter:        outputWriter,
	}

	if options.RateLimit > 0 {
		crawlerOptions.RateLimit = *ratelimit.New(context.Background(), uint(options.RateLimit), time.Second)
	} else if options.RateLimitMinute > 0 {
		crawlerOptions.RateLimit = *ratelimit.New(context.Background(), uint(options.RateLimitMinute), time.Minute)
	}

	wappalyze, err := wappalyzer.New()
	if err != nil {
		return nil, err
	}
	crawlerOptions.Wappalyzer = wappalyze

	return crawlerOptions, nil
}

// Close closes the crawler options resources
func (c *CrawlerOptions) Close() error {
	c.UniqueFilter.Close()
	return c.OutputWriter.Close()
}
