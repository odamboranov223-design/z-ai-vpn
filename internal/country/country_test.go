package country

import "testing"

func TestDetectFromFlagEmoji(t *testing.T) {
	cases := []struct {
		name  string
		input string
		code  string
	}{
		{"germany", "🇩🇪 Germany #1", "DE"},
		{"netherlands", "🇳🇱 Amsterdam", "NL"},
		{"finland", "🇫🇮 Helsinki", "FI"},
		{"sweden", "🇸🇪 Stockholm", "SE"},
	}
	for _, tc := range cases {
		code, name := Detect(tc.input)
		if code != tc.code {
			t.Errorf("%s: code = %q, want %q", tc.name, code, tc.code)
		}
		if name == "" {
			t.Errorf("%s: name is empty", tc.name)
		}
	}
}

func TestDetectIgnoresBrokenFlag(t *testing.T) {
	// Two regional indicators that do not form a country code.
	if code, _ := Detect("🇷🇮 Nowhere"); code != "" {
		t.Errorf("code = %q, want an empty code", code)
	}
}

func TestDetectFromCountryName(t *testing.T) {
	code, name := Detect("Германия Premium")
	if code != "DE" {
		t.Errorf("code = %q, want DE", code)
	}
	if name != "Германия" {
		t.Errorf("name = %q", name)
	}
}

func TestDetectFromCodeToken(t *testing.T) {
	code, _ := Detect("DE 1")
	if code != "DE" {
		t.Errorf("code = %q, want DE", code)
	}
}

func TestDetectUnknown(t *testing.T) {
	code, name := Detect("some random node 42")
	if code != "" || name != "" {
		t.Errorf("code = %q, name = %q, want both empty", code, name)
	}
}

func TestEmojiAndFlagURLOnlyForKnownCodes(t *testing.T) {
	if got := Emoji("DE"); got != "🇩🇪" {
		t.Errorf("Emoji(DE) = %q", got)
	}
	if got := Emoji("ZZ"); got != "🏳️" {
		t.Errorf("Emoji(ZZ) = %q, want the placeholder", got)
	}
	if got := FlagURL("ZZ"); got != "" {
		t.Errorf("FlagURL(ZZ) = %q, want an empty string", got)
	}
	if got := FlagURL("de"); got != "https://flagcdn.com/w40/de.png" {
		t.Errorf("FlagURL(de) = %q", got)
	}
}
