package backend

import "testing"

func TestGetVersion(t *testing.T) {
	svc := NewAppService("1.2.3")
	if got := svc.GetVersion(); got != "1.2.3" {
		t.Errorf("GetVersion() = %q, want %q", got, "1.2.3")
	}
}

func TestValidVersionPattern(t *testing.T) {
	valid := []string{"0.0.1", "1.2.3", "10.20.30", "999.999.999"}
	for _, v := range valid {
		t.Run("valid/"+v, func(t *testing.T) {
			if !validVersionPattern.MatchString(v) {
				t.Errorf("expected %q to match", v)
			}
		})
	}

	invalid := []string{"", "1", "1.2", "1.2.3.4", "v1.2.3", "1.2.3-beta", "abc", "1.2.x", " 1.2.3", "1.2.3 "}
	for _, v := range invalid {
		label := v
		if label == "" {
			label = "(empty)"
		}
		t.Run("invalid/"+label, func(t *testing.T) {
			if validVersionPattern.MatchString(v) {
				t.Errorf("expected %q to NOT match", v)
			}
		})
	}
}
