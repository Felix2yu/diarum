package store

import "testing"

func TestSystemSettingsCRUD(t *testing.T) {
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer s.Close()

	if _, ok, err := s.GetSystemSetting("holidays.cn.enabled"); ok || err != nil {
		t.Fatalf("unexpected initial value: ok=%v err=%v", ok, err)
	}

	if err := s.SetSystemSetting("holidays.cn.enabled", "true"); err != nil {
		t.Fatalf("SetSystemSetting: %v", err)
	}
	value, ok, err := s.GetSystemSetting("holidays.cn.enabled")
	if err != nil || !ok || value != "true" {
		t.Fatalf("GetSystemSetting = %q, ok=%v, err=%v", value, ok, err)
	}

	if err := s.SetSystemSetting("holidays.cn.enabled", "false"); err != nil {
		t.Fatalf("SetSystemSetting update: %v", err)
	}
	value, ok, err = s.GetSystemSetting("holidays.cn.enabled")
	if err != nil || !ok || value != "false" {
		t.Fatalf("GetSystemSetting after update = %q, ok=%v, err=%v", value, ok, err)
	}
}
