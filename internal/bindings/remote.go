package bindings

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// The remote file commands.
//
// The add dialog reads a torrent file that is somewhere else: the user
// gives an address, and the file is fetched so that the dialog can
// show what is in it before the download starts.

// The bounds of a fetched file.
const (
	// MaxRemoteFileSize is the largest a remote file may be.
	//
	// It is a torrent file, and a torrent file is a few hundred
	// kilobytes. The bound is there so that a wrong address cannot
	// fill the memory.
	MaxRemoteFileSize = 16 * 1024 * 1024
	// RemoteFileTimeout bounds the fetch.
	RemoteFileTimeout = 30 * time.Second
)

// RemoteRequestHeader is one header the request carries.
type RemoteRequestHeader struct {
	// Name is the name of the header.
	Name string `json:"name"`
	// Value is the value of the header.
	Value string `json:"value"`
}

// FetchRemoteBytes reads a file from an address.
//
// The file is read as it comes, and it is given to the frontend as the
// bytes it is.
func (s *AppService) FetchRemoteBytes(
	address string,
	proxy string,
	referer string,
	cookie string,
	userAgent string,
	requestHeaders []RemoteRequestHeader,
) ([]byte, error) {
	if strings.TrimSpace(address) == "" {
		return nil, fmt.Errorf("no address was given")
	}

	parsed, err := url.Parse(address)
	if err != nil {
		return nil, fmt.Errorf("the address is not valid: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("only http and https addresses are read")
	}

	ctx, cancel := context.WithTimeout(context.Background(), RemoteFileTimeout)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, fmt.Errorf("the request could not be built: %w", err)
	}

	// The headers the browser would have sent. The referer and the
	// cookie are what a server checks, and the user agent is what it
	// expects.
	if referer != "" {
		request.Header.Set("Referer", referer)
	}
	if cookie != "" {
		request.Header.Set("Cookie", cookie)
	}
	if userAgent != "" {
		request.Header.Set("User-Agent", userAgent)
	}
	for _, header := range requestHeaders {
		if header.Name == "" {
			continue
		}
		request.Header.Set(header.Name, header.Value)
	}

	client := &http.Client{Timeout: RemoteFileTimeout}
	if proxy != "" {
		proxyURL, err := url.Parse(proxy)
		if err != nil {
			return nil, fmt.Errorf("the proxy is not valid: %w", err)
		}
		client.Transport = &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("the file could not be fetched: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("the server answered HTTP %s", response.Status)
	}

	// The file is read as it comes, and the bound is checked as it is
	// read, so that a wrong address cannot fill the memory.
	var content []byte
	buffer := make([]byte, 32*1024)

	for {
		count, err := response.Body.Read(buffer)
		if count > 0 {
			if len(content)+count > MaxRemoteFileSize {
				return nil, fmt.Errorf("the file is larger than %d bytes", MaxRemoteFileSize)
			}
			content = append(content, buffer[:count]...)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("the file could not be read: %w", err)
		}
	}

	return content, nil
}

// SiteConnectionCap is the cap a site rule sets for the connections of
// a download.
//
// Some sites refuse a download that opens too many connections, so the
// rule lowers the number of them.
type SiteConnectionCap struct {
	// MaxConnectionPerServer is the number of connections the rule
	// allows.
	MaxConnectionPerServer int `json:"maxConnectionPerServer"`
	// HostPattern is the pattern of the hosts the rule applies to.
	HostPattern string `json:"hostPattern"`
}

// PreviewSiteRules answers what the site rules say about a set of
// addresses.
//
// It is what the add dialog shows when the user types an address that
// a rule applies to.
func (s *AppService) PreviewSiteRules(uris []string) (*SiteConnectionCap, error) {
	// The rules of the sites are a preference of the interface, and
	// BitMaher does not keep them yet.
	//
	// The answer is that no rule applies, which is the answer for
	// every address when there is no rule.
	for _, uri := range uris {
		if uri == "" {
			continue
		}
	}

	return nil, nil
}
