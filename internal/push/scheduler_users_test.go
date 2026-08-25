package push

import (
	"testing"

	"github.com/songtianlun/diarum/internal/config"
	"github.com/songtianlun/diarum/internal/store"
)

func TestSchedulerListUserIDs(t *testing.T) {
	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	u, err := s.CreateUser("pushuser", "push@example.com", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	sc := NewScheduler(s, config.NewConfigService(s), NewSender(s))
	ids, err := sc.listUserIDs()
	if err != nil {
		t.Fatalf("listUserIDs: %v", err)
	}
	if len(ids) != 1 || ids[0] != u.ID {
		t.Fatalf("listUserIDs = %v, want [%s]", ids, u.ID)
	}
}
