package backup

import (
	"testing"
)

func TestMakeLegalAlias(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantAlias string
		wantLegal bool
	}{
		{
			name:      "empty string returns escaped null and is illegal",
			input:     "",
			wantAlias: "%00",
			wantLegal: false,
		},
		{
			name:      "fully legal string is returned unchanged",
			input:     "backup-2024.01.02_full+db",
			wantAlias: "backup-2024.01.02_full+db",
			wantLegal: true,
		},
		{
			name:      "a space is escaped but remains legal",
			input:     "my backup",
			wantAlias: "my%20backup",
			wantLegal: true,
		},
		{
			name:      "multiple spaces stay legal",
			input:     "a b c",
			wantAlias: "a%20b%20c",
			wantLegal: true,
		},
		{
			name:      "slash is escaped and marks the alias illegal",
			input:     "a/b",
			wantAlias: "a%2Fb",
			wantLegal: false,
		},
		{
			name:      "leading illegal character is escaped",
			input:     "/leading",
			wantAlias: "%2Fleading",
			wantLegal: false,
		},
		{
			name:      "mix of space and illegal char is illegal overall",
			input:     "a b/c",
			wantAlias: "a%20b%2Fc",
			wantLegal: false,
		},
		{
			name:      "reserved characters are each escaped",
			input:     "?&=",
			wantAlias: "%3F%26%3D",
			wantLegal: false,
		},
		{
			name:      "characters above z are escaped",
			input:     "a{b",
			wantAlias: "a%7Bb",
			wantLegal: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotAlias, gotLegal := MakeLegalAlias(tt.input)
			if gotAlias != tt.wantAlias {
				t.Errorf("MakeLegalAlias(%q) alias = %q, want %q", tt.input, gotAlias, tt.wantAlias)
			}
			if gotLegal != tt.wantLegal {
				t.Errorf("MakeLegalAlias(%q) legal = %v, want %v", tt.input, gotLegal, tt.wantLegal)
			}
		})
	}
}

func TestToHex(t *testing.T) {
	tests := []struct {
		in     byte
		wantHi byte
		wantLo byte
	}{
		{in: 0x00, wantHi: '0', wantLo: '0'},
		{in: ' ', wantHi: '2', wantLo: '0'}, // 0x20
		{in: '/', wantHi: '2', wantLo: 'F'}, // 0x2F
		{in: 0xFF, wantHi: 'F', wantLo: 'F'},
		{in: 0xAB, wantHi: 'A', wantLo: 'B'},
	}

	for _, tt := range tests {
		hi, lo := toHex(tt.in)
		if hi != tt.wantHi || lo != tt.wantLo {
			t.Errorf("toHex(%#x) = %q%q, want %q%q", tt.in, hi, lo, tt.wantHi, tt.wantLo)
		}
	}
}

func TestIsLegalInUrl(t *testing.T) {
	legal := []byte{'a', 'z', 'A', 'Z', '0', '9', '-', '.', '_', '+', '!', '$'}
	for _, c := range legal {
		if !isLegalInUrl(c) {
			t.Errorf("isLegalInUrl(%q) = false, want true", c)
		}
	}

	illegal := []byte{' ', '"', '#', '%', '&', '/', ':', ';', '<', '=', '>', '?', '@', '[', '\\', ']', '^', '`', '{', '|', '}', '~', 0x00}
	for _, c := range illegal {
		if isLegalInUrl(c) {
			t.Errorf("isLegalInUrl(%q) = true, want false", c)
		}
	}
}
