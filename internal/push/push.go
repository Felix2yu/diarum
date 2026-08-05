// Package push implements Web Push (RFC 8291 / RFC 8292) notification sending
// using VAPID-authenticated encrypted messages, backed by a SQLite-stored
// subscription registry.
package push

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/SherClockHolmes/webpush-go"
	"github.com/songtianlun/diarum/internal/logger"
	"github.com/songtianlun/diarum/internal/store"
)

// SubscriberEmail is the "sub" claim used in the VAPID JWT token. Push services
// require a contact; admins may override via SetSubscriberEmail.
var SubscriberEmail = "diarum@localhost"

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

// SendNotification sends a push notification to all subscriptions owned by the user.
// Subscriptions that return 410 Gone are removed from the store.
func (s *Sender) SendNotification(owner, title, body string) error {
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

	var client = &http.Client{Timeout: 15 * time.Second}
	for _, sub := range subs {
		wpSub := &webpush.Subscription{
			Endpoint: sub.Endpoint,
			Keys: webpush.Keys{
				Auth:   sub.Auth,
				P256dh: sub.P256dh,
			},
		}
		resp, err := webpush.SendNotificationWithContext(context.Background(), payload, wpSub, &webpush.Options{
			HTTPClient:      client,
			Subscriber:      SubscriberEmail,
			TTL:             60,
			VAPIDPublicKey:  s.pubKey,
			VAPIDPrivateKey: s.privKey,
		})
		if err != nil {
			logger.Warn("[Push] failed to send to %s: %v", sub.Endpoint, err)
			continue
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusNotFound {
			logger.Info("[Push] subscription expired (%d), removing %s", resp.StatusCode, sub.Endpoint)
			_ = s.store.DeletePushSubscription(owner, sub.Endpoint)
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			logger.Warn("[Push] push service returned %d for %s", resp.StatusCode, sub.Endpoint)
			continue
		}
		logger.Debug("[Push] sent notification to %s", sub.Endpoint)
	}
	return nil
}
