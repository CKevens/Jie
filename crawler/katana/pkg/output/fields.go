package output

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/net/publicsuffix"
)

// FieldNames is a list of supported field names
var FieldNames = []string{
	"url",
	"path",
	"fqdn",
	"rdn",
	"rurl",
	"qurl",
	"qpath",
	"file",
	"key",
	"value",
	"kv",
	"dir",
	"udir",
}

// validateFieldNames validates provided field names
func validateFieldNames(names string) error {
	parts := strings.Split(names, ",")
	if len(parts) == 0 {
		return errors.Errorf("no field names provided: %s", names)
	}
	uniqueFields := make(map[string]struct{})
	for _, field := range FieldNames {
		uniqueFields[field] = struct{}{}
	}
	for _, part := range parts {
		if _, ok := uniqueFields[part]; !ok {
			return errors.Errorf("invalid field %s specified: %s", part, names)
		}
	}
	return nil
}

// storeFields stores fields for a result into individual files
// based on name.
func storeFields(output *Result, storeFields []string) {
	parsed, err := url.Parse(output.URL)
	if err != nil {
		return
	}

	hostname := parsed.Hostname()
	etld, _ := publicsuffix.EffectiveTLDPlusOne(hostname)
	rootURL := fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)
	for _, field := range storeFields {
		if result := getValueForField(output, parsed, hostname, etld, rootURL, field); result != "" {
			appendToFileField(parsed, field, result)
		}
	}
}

func appendToFileField(parsed *url.URL, field, data string) {
	file, err := os.OpenFile(path.Join(storeFieldsDirectory, fmt.Sprintf("%s_%s_%s.txt", parsed.Scheme, parsed.Hostname(), field)), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return
	}
	defer file.Close()

	_, _ = file.WriteString(data)
	_, _ = file.Write([]byte("\n"))
}

// formatField formats output results based on fields from fieldNames
func formatField(output *Result, fields string) string {
	parsed, _ := url.Parse(output.URL)
	if parsed == nil {
		return ""
	}

	queryLen := len(parsed.Query())
	queryBoth := make([]string, 0, queryLen)
	queryKeys := make([]string, 0, queryLen)
	queryValues := make([]string, 0, queryLen)
	if queryLen > 0 {
		for k, v := range parsed.Query() {
			for _, value := range v {
				queryBoth = append(queryBoth, strings.Join([]string{k, value}, "="))
			}
			queryKeys = append(queryKeys, k)
			queryValues = append(queryValues, v...)
		}
	}
	hostname := parsed.Hostname()
	etld, _ := publicsuffix.EffectiveTLDPlusOne(hostname)
	rootURL := fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)
	values := []string{
		"url", output.URL,
		"rurl", rootURL,
		"rdn", etld,
		"path", parsed.Path,
		"fqdn", hostname,
	}
	if len(queryKeys) > 0 {
		values = append(values, "qurl", output.URL)
		values = append(values, "qpath", fmt.Sprintf("%s?%s", parsed.Path, parsed.Query().Encode()))
	} else {
		values = append(values, "qurl", "")
		values = append(values, "qpath", "")
	}
	if len(queryKeys) > 0 || len(queryValues) > 0 || len(queryBoth) > 0 {
		values = append(values, "key", strings.Join(queryKeys, "\n"))
		values = append(values, "kv", strings.Join(queryBoth, "\n"))
		values = append(values, "value", strings.Join(queryValues, "\n"))
	}
	if parsed.Path != "" && parsed.Path != "/" {
		basePath := path.Base(parsed.Path)
		if strings.Contains(basePath, ".") {
			values = append(values, "file", basePath)
		}
		if strings.Contains(parsed.Path[1:], "/") {
			directory := parsed.Path[:strings.LastIndex(parsed.Path[1:], "/")+2]
			values = append(values, "udir", fmt.Sprintf("%s%s", rootURL, directory))
			values = append(values, "dir", directory)
		}
	}
	replacer := strings.NewReplacer(values...)
	replaced := replacer.Replace(fields)
	if replaced == fields {
		return ""
	}
	return replaced
}

// getValueForField returns value for a field
func getValueForField(output *Result, parsed *url.URL, hostname, rdn, rurl, field string) string {
	switch field {
	case "url":
		return output.URL
	case "path":
		return parsed.Path
	case "fqdn":
		return hostname
	case "rdn":
		return rdn
	case "rurl":
		return rurl
	case "file":
		basePath := path.Base(parsed.Path)
		if parsed.Path != "" && parsed.Path != "/" && strings.Contains(basePath, ".") {
			return basePath
		}
	case "dir":
		if parsed.Path != "" && parsed.Path != "/" && strings.Contains(parsed.Path[1:], "/") {
			return parsed.Path[:strings.LastIndex(parsed.Path[1:], "/")+2]
		}
	case "udir":
		if parsed.Path != "" && parsed.Path != "/" && strings.Contains(parsed.Path[1:], "/") {
			return fmt.Sprintf("%s%s", rurl, parsed.Path[:strings.LastIndex(parsed.Path[1:], "/")+2])
		}
	case "qpath":
		if len(parsed.Query()) > 0 {
			return fmt.Sprintf("%s?%s", parsed.Path, parsed.Query().Encode())
		}
	case "qurl":
		if len(parsed.Query()) > 0 {
			return parsed.String()
		}
	case "key":
		values := make([]string, 0, len(parsed.Query()))
		for k := range parsed.Query() {
			values = append(values, k)
		}
		return strings.Join(values, "\n")
	case "value":
		values := make([]string, 0, len(parsed.Query()))
		for _, v := range parsed.Query() {
			values = append(values, v...)
		}
		return strings.Join(values, "\n")
	case "kv":
		values := make([]string, 0, len(parsed.Query()))
		for k, v := range parsed.Query() {
			for _, value := range v {
				values = append(values, strings.Join([]string{k, value}, "="))
			}
		}
		return strings.Join(values, "\n")
	}
	return ""
}
