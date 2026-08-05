// Package push implements Web Push (RFC 8291 / RFC 8292) notification sending
// using VAPID-authenticated encrypted messages, backed by a SQLite-stored
// subscription registry.
package push

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/SherClockHolmes/webpush-go"
	"github.com/songtianlun/diarum/internal/logger"
	"github.com/songtianlun/diarum/internal/store"
)

// SubscriberEmail is the fallback "sub" claim used in the VAPID JWT token when
// no usable deployment hostname or override is available. Apple's Web Push
// service rejects subjects that are not a valid URL or mailto: address with a
// public email domain, so a caller that relies on this fallback will be
// rejected by Apple (it still works with Chrome/Firefox push services).
var SubscriberEmail = "diarum@localhost"

// SubscriberOverride, when set, is used verbatim as the VAPID "sub" claim.
// Admins can pin a valid contact (e.g. via the DIARUM_PUSH_SUBSCRIBER env var)
// when the deployment host cannot be auto-detected reliably.
var SubscriberOverride string

// SiteHost records the deployment hostname (e.g. "diarum.example.com") seen on
// the latest request. It is used to build the VAPID subject and the Topic
// header, which Apple's push service expects to match the subscribed origin.
// It is populated by the push API middleware.
var SiteHost string

// SiteOrigin records the full Origin (e.g. "https://diarum.example.com") seen
// on the latest request. It is preferred over SiteHost because reverse proxies
// often rewrite the Host header but preserve the browser's Origin, which is
// the true public origin that Safari subscribed under.
var SiteOrigin string

// NormalizeHost strips the port and lowercases a Host header value.
func NormalizeHost(host string) string {
	h := strings.ToLower(strings.TrimSpace(host))
	if i := strings.Index(h, ":"); i >= 0 {
		h = h[:i]
	}
	return h
}

// OriginHost extracts a normalized hostname from an Origin/URL string.
func OriginHost(origin string) string {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return ""
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return ""
	}
	return NormalizeHost(u.Host)
}

// isPublicHost reports whether host is usable as a VAPID subject domain.
// Apple rejects mailto: subjects whose domain is not a real public domain
// (e.g. "localhost", IP addresses, ".local" mDNS names).
func isPublicHost(host string) bool {
	if host == "" || host == "localhost" {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return false
	}
	if strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".lan") {
		return false
	}
	return true
}

// Sender manages VAPID keys and sends push notifications to stored subscriptions.
type Sender struct {
	store *store.Store

	mu      sync.Mutex
	loaded  bool
	pubKey  string
	privKey string
}

// NewSender creates a push Sender backed by the given store.
func NewSender(s *store.Store) *Sender {
	return &Sender{store: s}
}

// EnsureVAPIDKeys generates and persists a VAPID key pair on first use.
func (s *Sender) EnsureVAPIDKeys() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.loaded {
		return nil
	}

	pub, err := s.store.GetVAPIDKey("public")
	if err != nil || pub == "" {
		// generate a fresh pair and persist it
		priv, pubKey, genErr := webpush.GenerateVAPIDKeys()
		if genErr != nil {
			return fmt.Errorf("generate vapid keys: %w", genErr)
		}
		if setErr := s.store.SetVAPIDKey("public", pubKey); setErr != nil {
			return fmt.Errorf("persist vapid public key: %w", setErr)
		}
		if setErr := s.store.SetVAPIDKey("private", priv); setErr != nil {
			return fmt.Errorf("persist vapid private key: %w", setErr)
		}
		s.pubKey = pubKey
		s.privKey = priv
		s.loaded = true
		logger.Info("[Push] generated new VAPID key pair")
		return nil
	}

	priv, err := s.store.GetVAPIDKey("private")
	if err != nil || priv == "" {
		return fmt.Errorf("vapid public key exists but private key is missing")
	}

	s.pubKey = pub
	s.privKey = priv
	s.loaded = true
	return nil
}

// PublicKey returns the base64url-encoded VAPID public key.
func (s *Sender) PublicKey() (string, error) {
	if err := s.EnsureVAPIDKeys(); err != nil {
		return "", err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pubKey, nil
}

// subscriber returns the VAPID "sub" claim. It prefers an explicitly pinned
// override, then a mailto: derived from the site origin/host. Apple's push
// service rejects subjects that are not a URL or mailto: with a real public
// email domain.
func (s *Sender) subscriber() string {
	if SubscriberOverride != "" {
		return SubscriberOverride
	}
	for _, candidate := range []string{OriginHost(SiteOrigin), NormalizeHost(SiteHost)} {
		if isPublicHost(candidate) {
			return "mailto:webpush@" + candidate
		}
	}
	return SubscriberEmail
}

// SendNotification sends a push notification to all subscriptions owned by the user.
// Subscriptions that return 410 Gone are removed from the store.
func (s *Sender) SendNotification(owner, title, body string) error {
	return s.SendNotificationWithClient(owner, title, body, &http.Client{Timeout: 15 * time.Second})
}

// SendNotificationWithClient is SendNotification with an explicit HTTP client
// (used by tests to route requests through a custom transport).
func (s *Sender) SendNotificationWithClient(owner, title, body string, client *http.Client) error {
	// Guarantee the VAPID key pair is loaded before sending, no matter which
	// Sender instance or code path triggered the notification.
	if err := s.EnsureVAPIDKeys(); err != nil {
		return fmt.Errorf("ensure vapid keys: %w", err)
	}
	s.mu.Lock()
	pubKey := s.pubKey
	privKey := s.privKey
	s.mu.Unlock()

	subs, err := s.store.ListPushSubscriptions(owner)
	if err != nil {
		return err
	}
	if len(subs) == 0 {
		return nil
	}

	payload, err := json.Marshal(map[string]string{"title": title, "body": body})
	if err != nil {
		return err
	}

	// Apple's push service requires the Topic header to match the subscribed
	// origin and rejects requests without it. Other push services ignore it.
	topic := NormalizeHost(SiteHost)
	for _, sub := range subs {
		wpSub := &webpush.Subscription{
			Endpoint: sub.Endpoint,
			Keys: webpush.Keys{
				Auth:   sub.Auth,
				P256dh: sub.P256dh,
			},
		}
		opts := &webpush.Options{
			HTTPClient:      client,
			Subscriber:      s.subscriber(),
			TTL:             60,
			VAPIDPublicKey:  pubKey,
			VAPIDPrivateKey: privKey,
		}
		if strings.Contains(sub.Endpoint, "web.push.apple.com") && topic != "" {
			opts.Topic = topic
		}
		resp, err := webpush.SendNotificationWithContext(context.Background(), payload, wpSub, opts)
		if err != nil {
			logger.Warn("[Push] failed to send to %s: %v", sub.Endpoint, err)
			continue
		}
		reason, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		resp.Body.Close()
		if resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusNotFound {
			logger.Info("[Push] subscription expired (%d), removing %s", resp.StatusCode, sub.Endpoint)
			_ = s.store.DeletePushSubscription(owner, sub.Endpoint)
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			logger.Warn("[Push] push service returned %d for %s: %s", resp.StatusCode, sub.Endpoint, strings.TrimSpace(string(reason)))
			continue
		}
		logger.Debug("[Push] sent notification to %s", sub.Endpoint)
	}
	return nil
}
