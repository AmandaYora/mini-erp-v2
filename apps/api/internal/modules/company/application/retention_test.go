package application

import "testing"

func TestRetentionSettingsValidated(t *testing.T) {
	out, err := normalizeSettings(map[string]string{SettingRetentionAudit: " 365 ", SettingRetentionSession: "30"})
	if err != nil || out[SettingRetentionAudit] != "365" {
		t.Fatalf("valid: %v %v", out, err)
	}
	for _, v := range []string{"0", "-1", "abc", "1.5", "99999"} {
		if _, err := normalizeSettings(map[string]string{SettingRetentionAudit: v}); err == nil {
			t.Errorf("%q must be rejected", v)
		}
	}
}
