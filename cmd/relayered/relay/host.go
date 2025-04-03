package relay

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/bluesky-social/indigo/atproto/syntax"
	"github.com/bluesky-social/indigo/cmd/relayered/relay/models"

	"gorm.io/gorm"
)

func (r *Relay) GetHost(ctx context.Context, hostID uint64) (*models.Host, error) {
	ctx, span := tracer.Start(ctx, "getHost")
	defer span.End()

	var host models.Host
	if err := r.db.Find(&host, hostID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrHostNotFound
		}
		return nil, err
	}

	// TODO: is this further check needed?
	if host.ID == 0 {
		return nil, ErrAccountNotFound
	}

	return &host, nil
}

// parses, normalizes, and validates a raw URL (HTTP or WebSocket) in to a hostname for subscriptions
//
// Hostnames much be DNS names, not IP addresses
func ParseHostname(raw string) (hostname string, noSSL bool, err error) {
	u, err := url.Parse(raw)
	noSSL = false
	switch u.Scheme {
	case "https", "wss":
		// pass
	case "http", "ws":
		noSSL = true
	default:
		return "", false, fmt.Errorf("unsupported URL scheme: %s", u.Scheme)
	}
	// 'localhost' (exact string) is allowed *with* a required port number; SSL is optional
	if u.Hostname() == "localhost" {
		if u.Port() == "" {
			return "", false, fmt.Errorf("port number is required for localhost")
		}
		return u.Host, noSSL, nil
	}

	// port numbers not allowed otherwise
	if u.Port() != "" {
		return "", false, fmt.Errorf("port number not allowed for non-local names")
	}

	// check it is a real hostname (eg, not IP address or single-word alias)
	// TODO: more SSRF protection here? eg disallow '.local'
	h, err := syntax.ParseHandle(u.Host)
	if err != nil {
		return "", false, fmt.Errorf("not a public hostname")
	}
	// lower-case in reponse
	return h.Normalize().String(), noSSL, nil
}

func IsTrustedHostname(hostname string, domains []string) bool {
	for _, d := range domains {
		if hostname == d {
			return true
		}
		if strings.HasPrefix(d, "*") && strings.HasSuffix(hostname, d[1:]) {
			return true
		}
	}
	return false
}
