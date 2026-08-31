package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sspzoa/goppi/internal/provider"
	"github.com/sspzoa/goppi/internal/upstage"
)

const maxFetchBytes = 256 << 10

type webFetch struct {
	client *http.Client
}

func (webFetch) Spec() provider.ToolSpec {
	return provider.ToolSpec{
		Name: "web_fetch",
		Description: "Fetch a public http(s) URL and return text. Use for docs, GitHub, and API references. " +
			"Not a browser: no JavaScript. Do not use for file://, localhost, private IPs, or credential stuffing.",
		Parameters: schema(`{
			"type":"object",
			"properties":{
				"url":{"type":"string","description":"http or https URL"}
			},
			"required":["url"]
		}`),
	}
}

func (t webFetch) Run(ctx context.Context, input json.RawMessage) (string, error) {
	args, err := decode[struct {
		URL string `json:"url"`
	}](input)
	if err != nil {
		return "", err
	}
	u, err := url.Parse(strings.TrimSpace(args.URL))
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("invalid url")
	}
	if u.User != nil {
		return "", fmt.Errorf("url must not include credentials")
	}
	if err := checkFetchURL(u); err != nil {
		return "", err
	}
	return t.get(ctx, u)
}

func (t webFetch) get(ctx context.Context, u *url.URL) (string, error) {
	client := t.client
	if client == nil {
		client = fetchClient()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("user-agent", "goppi")
	req.Header.Set("accept", "text/plain, text/markdown, text/html, application/json;q=0.9, */*;q=0.1")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFetchBytes+1))
	if err != nil {
		return "", err
	}
	if len(body) > maxFetchBytes {
		body = body[:maxFetchBytes]
	}
	text := strings.TrimSpace(string(body))
	if strings.Contains(strings.ToLower(resp.Header.Get("content-type")), "html") {
		text = stripTags(text)
	}
	if text == "" {
		text = "(empty body)"
	}
	return fmt.Sprintf("HTTP %d %s\n%s", resp.StatusCode, u.String(), text), nil
}

func fetchClient() *http.Client {
	client := upstage.NewHTTPClient(20 * time.Second)
	if tr, ok := client.Transport.(*http.Transport); ok {
		tr = tr.Clone()
		tr.DialContext = safeDial
		client.Transport = tr
	}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("too many redirects")
		}
		if req.URL.User != nil {
			return fmt.Errorf("url must not include credentials")
		}
		return checkFetchURL(req.URL)
	}
	return client
}

func safeDial(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	var ips []net.IP
	if ip := net.ParseIP(host); ip != nil {
		ips = []net.IP{ip}
	} else {
		ips, err = lookupIP(host)
		if err != nil {
			return nil, fmt.Errorf("lookup %s: %w", host, err)
		}
	}
	d := &net.Dialer{Timeout: 10 * time.Second}
	var last error
	for _, ip := range ips {
		if err := blockedIP(ip); err != nil {
			last = err
			continue
		}
		c, err := d.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err != nil {
			last = err
			continue
		}
		if tcp, ok := c.RemoteAddr().(*net.TCPAddr); ok {
			if err := blockedIP(tcp.IP); err != nil {
				_ = c.Close()
				last = err
				continue
			}
		}
		return c, nil
	}
	if last != nil {
		return nil, last
	}
	return nil, fmt.Errorf("refusing private or local host")
}

var lookupIP = net.LookupIP

func checkFetchURL(u *url.URL) error {
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
	default:
		return fmt.Errorf("only http(s) urls are allowed")
	}
	return checkFetchHost(u.Hostname())
}

func checkFetchHost(host string) error {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return fmt.Errorf("invalid url")
	}
	if err := blockedName(host); err != nil {
		return err
	}
	if ip := net.ParseIP(host); ip != nil {
		return blockedIP(ip)
	}
	ips, err := lookupIP(host)
	if err != nil {
		return fmt.Errorf("lookup %s: %w", host, err)
	}
	for _, ip := range ips {
		if err := blockedIP(ip); err != nil {
			return err
		}
	}
	return nil
}

func blockedName(host string) error {
	if metadataName(host) {
		return fmt.Errorf("refusing metadata host")
	}
	if host == "localhost" || host == "localhost.localdomain" || strings.HasSuffix(host, ".localhost") {
		return fmt.Errorf("refusing private or local host")
	}
	return nil
}

func metadataName(host string) bool {
	switch host {
	case "metadata.google.internal", "metadata.goog", "metadata.azure.com":
		return true
	default:
		return false
	}
}

func blockedIP(ip net.IP) error {
	if ip == nil {
		return fmt.Errorf("invalid url")
	}
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return fmt.Errorf("refusing metadata host")
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsMulticast() || cgnat(ip) {
		return fmt.Errorf("refusing private or local host")
	}
	return nil
}

func cgnat(ip net.IP) bool {
	ip4 := ip.To4()
	return ip4 != nil && ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127
}

func stripTags(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		switch {
		case r == '<':
			in = true
		case r == '>':
			in = false
			b.WriteByte(' ')
		case !in:
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
