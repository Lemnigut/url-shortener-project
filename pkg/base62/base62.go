package base62

import (
	"math"
	"strings"
)

const charset = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
const base = 62

// Encode конвертирует положительное целое число в строку base62.
func Encode(n int64) string {
	if n == 0 {
		return string(charset[0])
	}

	var sb strings.Builder
	for n > 0 {
		sb.WriteByte(charset[n%base])
		n /= base
	}

	// Разворачиваем строку
	result := []byte(sb.String())
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return string(result)
}

// Decode конвертирует строку base62 обратно в целое число.
func Decode(s string) (int64, error) {
	var n int64
	for i, c := range s {
		idx := strings.IndexRune(charset, c)
		if idx < 0 {
			return 0, &InvalidCharError{Char: c, Pos: i}
		}
		n = n*base + int64(idx)
		if n < 0 {
			return 0, ErrOverflow
		}
	}
	return n, nil
}

// MaxLength возвращает максимальную длину base62-строки для заданной битности.
func MaxLength(bits int) int {
	return int(math.Ceil(float64(bits) * math.Log(2) / math.Log(float64(base))))
}

// InvalidCharError — недопустимый символ в base62-строке.
type InvalidCharError struct {
	Char rune
	Pos  int
}

func (e *InvalidCharError) Error() string {
	return "base62: недопустимый символ"
}

// ErrOverflow — переполнение при декодировании.
var ErrOverflow = &overflowError{}

type overflowError struct{}

func (e *overflowError) Error() string {
	return "base62: переполнение числа"
}
