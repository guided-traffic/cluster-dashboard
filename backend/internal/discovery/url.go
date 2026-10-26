package discovery

import (
	"errors"
	"net/url"
	"strings"

	networkingv1 "k8s.io/api/networking/v1"
)

// errNoHost is why an Ingress without a dash.gtrfc.com/url is skipped when
// no rule names a host a browser can open.
var errNoHost = errors.New("no rule names a concrete host; dash.gtrfc.com/url can name the address")

// regexpChars mark a path written for a controller's regular-expression
// matching: no browser can open it as it is.
const regexpChars = `^$*+?()[]{}|\`

// derivedURL is the address of an Ingress (docs/adr/0007 D3), where Hajimari
// takes tls[0].hosts[0] and the first rule's first path: the first rule with
// a concrete host — not empty, not a wildcard; https when that host is in
// any TLS block, a wildcard there covering it; the rule's first Prefix or
// Exact path without regular-expression characters, else "/".
func derivedURL(spec networkingv1.IngressSpec) (string, error) {
	for _, rule := range spec.Rules {
		if rule.Host == "" || strings.Contains(rule.Host, "*") {
			continue
		}
		u := url.URL{Scheme: "http", Host: rule.Host, Path: firstPath(rule)}
		if hasTLS(spec.TLS, rule.Host) {
			u.Scheme = "https"
		}
		return u.String(), nil
	}
	return "", errNoHost
}

func firstPath(rule networkingv1.IngressRule) string {
	if rule.HTTP == nil {
		return "/"
	}
	for _, p := range rule.HTTP.Paths {
		if p.PathType == nil || (*p.PathType != networkingv1.PathTypePrefix && *p.PathType != networkingv1.PathTypeExact) {
			continue
		}
		if !strings.HasPrefix(p.Path, "/") || strings.ContainsAny(p.Path, regexpChars) {
			return "/"
		}
		return p.Path
	}
	return "/"
}

// hasTLS reports whether a TLS block names host, itself or by a wildcard
// for its first label.
func hasTLS(blocks []networkingv1.IngressTLS, host string) bool {
	_, parentDomain, _ := strings.Cut(host, ".")
	for _, block := range blocks {
		for _, h := range block.Hosts {
			if strings.EqualFold(h, host) || (parentDomain != "" && strings.EqualFold(h, "*."+parentDomain)) {
				return true
			}
		}
	}
	return false
}

// overrideURL reads dash.gtrfc.com/url: an absolute http or https URL, which
// Hajimari's url.ParseRequestURI would take with any scheme. Its error never
// names the value.
func overrideURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return "", errors.New("not an absolute http or https URL")
	}
	if u.User != nil {
		return "", errors.New("holds a user or password, which every viewer would see")
	}
	return u.String(), nil
}
