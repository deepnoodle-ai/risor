package casefile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
)

// DecodeJSON reads one JSON value. Objects become map[string]any, arrays
// []any, numbers float64, and the rest string, bool and nil. An object that
// repeats a key, trailing text, and a number that reads as an infinity are
// errors (03-host.md 3.10.10 items 1 and 13). Numbers are read by the rule
// of text to number of 01-language.md 6.3, which is the JSON grammar.
func DecodeJSON(data []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	v, err := decodeValue(dec, 1)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("text after the JSON value")
	}
	return v, nil
}

// MaxDepth is how deep the JSON of a case file may nest (3.10.10 item 15).
const MaxDepth = 1024

func decodeValue(dec *json.Decoder, depth int) (any, error) {
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		if depth > MaxDepth {
			return nil, fmt.Errorf("JSON nested deeper than %d levels", MaxDepth)
		}
		switch t {
		case '{':
			m := map[string]any{}
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, err
				}
				k := kt.(string)
				if _, dup := m[k]; dup {
					return nil, fmt.Errorf("the key %q is repeated", k)
				}
				v, err := decodeValue(dec, depth+1)
				if err != nil {
					return nil, err
				}
				m[k] = v
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return m, nil
		case '[':
			a := []any{}
			for dec.More() {
				v, err := decodeValue(dec, depth+1)
				if err != nil {
					return nil, err
				}
				a = append(a, v)
			}
			if _, err := dec.Token(); err != nil {
				return nil, err
			}
			return a, nil
		}
		return nil, fmt.Errorf("unexpected %v", t)
	case json.Number:
		f, err := strconv.ParseFloat(string(t), 64)
		if err != nil || math.IsInf(f, 0) {
			return nil, fmt.Errorf("the number %s reads as an infinity", t)
		}
		if f == 0 {
			f = 0 // one zero: -0 reads as zero
		}
		return f, nil
	default:
		return t, nil // string, bool, nil
	}
}
