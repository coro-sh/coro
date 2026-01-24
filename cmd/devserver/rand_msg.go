package main

import (
	"encoding/json"
	"math/rand"
	"strings"
)

// randomMessage returns a random []byte representing one of:
//  1. Single-line text
//  2. Multi-line text
//  3. Single-line JSON
//  4. Multi-line (pretty) JSON
//  5. Binary / invalid UTF-8
//  6. Control-heavy but valid UTF-8
func randomMessage() []byte {
	switch rand.Intn(6) {
	case 0:
		return randomSingleLineText()
	case 1:
		return randomMultiLineText()
	case 2:
		return randomSingleLineJSON()
	case 3:
		return randomMultiLineJSON()
	case 4:
		return randomBinary()
	case 5:
		return randomControlUTF8()
	default:
		return []byte("fallback")
	}
}

func randomSingleLineText() []byte {
	sentences := []string{
		"hello world this is a test message",
		"the quick brown fox jumps over the lazy dog",
		"stream processing at scale is fun",
		"go makes concurrency pleasant",
		"short",
	}
	return []byte(sentences[rand.Intn(len(sentences))])
}

func randomMultiLineText() []byte {
	lines := []string{
		"this is line one",
		"and this is line two",
		"line three has more content",
		"the final line ends here",
	}

	n := rand.Intn(len(lines)-1) + 2
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteString(lines[rand.Intn(len(lines))])
		if rand.Intn(2) == 0 {
			b.WriteString("\n")
		} else {
			b.WriteString("\r\n")
		}
	}
	return []byte(b.String())
}

func randomSingleLineJSON() []byte {
	payload := map[string]any{
		"id":     rand.Intn(1000),
		"name":   "example",
		"active": rand.Intn(2) == 0,
	}
	b, _ := json.Marshal(payload)
	return b
}

func randomMultiLineJSON() []byte {
	payload := map[string]any{
		"id":   rand.Intn(1000),
		"user": "alice",
		"meta": map[string]any{
			"ip":      "127.0.0.1",
			"retries": rand.Intn(5),
			"flags": []string{
				"alpha",
				"beta",
				"gamma",
			},
		},
	}
	b, _ := json.MarshalIndent(payload, "", "  ")
	return b
}

func randomBinary() []byte {
	// Intentionally invalid UTF-8
	size := rand.Intn(32) + 8
	b := make([]byte, size)
	for i := range b {
		b[i] = byte(rand.Intn(256))
	}
	return b
}

func randomControlUTF8() []byte {
	// Valid UTF-8 but mostly non-printable
	return []byte{
		0x01, 0x02, 0x03, 0x04,
		0x05, 0x06, 0x07,
		0x08, 0x09, 0x0a,
	}
}
