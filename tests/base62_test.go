package tests

import (
	"testing"

	"tinyurl/pkg/base62"
)

func TestEncode(t *testing.T) {
	tests := []struct {
		name string
		in   int64
		want string
	}{
		{"ноль", 0, "0"},
		{"один", 1, "1"},
		{"десять", 10, "A"},
		{"шестьдесят_один", 61, "z"},
		{"шестьдесят_два", 62, "10"},
		{"большое_число", 123456789, "8M0kX"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := base62.Encode(tt.in)
			if got != tt.want {
				t.Errorf("Encode(%d) = %q, ожидалось %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestDecode(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		want    int64
		wantErr bool
	}{
		{"ноль", "0", 0, false},
		{"один", "1", 1, false},
		{"десять", "A", 10, false},
		{"шестьдесят_два", "10", 62, false},
		{"большое_число", "8M0kX", 123456789, false},
		{"недопустимый_символ", "abc!", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := base62.Decode(tt.in)
			if (err != nil) != tt.wantErr {
				t.Errorf("Decode(%q) ошибка = %v, ожидалась ошибка %v", tt.in, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("Decode(%q) = %d, ожидалось %d", tt.in, got, tt.want)
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	values := []int64{0, 1, 62, 1000, 999999, 123456789012, 1<<53 - 1}
	for _, v := range values {
		encoded := base62.Encode(v)
		decoded, err := base62.Decode(encoded)
		if err != nil {
			t.Fatalf("Decode(Encode(%d)) ошибка: %v", v, err)
		}
		if decoded != v {
			t.Errorf("RoundTrip(%d): encoded=%q, decoded=%d", v, encoded, decoded)
		}
	}
}

func TestMaxLength(t *testing.T) {
	got := base62.MaxLength(64)
	if got != 11 {
		t.Errorf("MaxLength(64) = %d, ожидалось 11", got)
	}
}
