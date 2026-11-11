package casefile

import "fmt"

// CaseSpan is where a case object stands in the text of a file, for the
// text update mode writes (package F9 4.5). Offsets are of bytes.
type CaseSpan struct {
	// ExpectStart and ExpectEnd bound the value of the member expect; both
	// are -1 when the case has none.
	ExpectStart, ExpectEnd int
	// LastEnd is the end of the value of the object's last member.
	LastEnd int
}

// CaseSpans finds the case objects of a valid JSON case file, in order.
func CaseSpans(text []byte) ([]CaseSpan, error) {
	s := &scanner{b: text}
	s.ws()
	switch s.peek() {
	case '[':
		return s.caseArray()
	case '{':
		var out []CaseSpan
		var err error
		found := false
		_, err = s.object(func(key string) error {
			if key != "cases" {
				return s.skip()
			}
			found = true
			s.ws()
			out, err = s.caseArray()
			return err
		})
		if err == nil && !found {
			err = fmt.Errorf("no cases")
		}
		return out, err
	}
	return nil, fmt.Errorf("not a case file")
}

type scanner struct {
	b []byte
	i int
}

func (s *scanner) peek() byte {
	if s.i < len(s.b) {
		return s.b[s.i]
	}
	return 0
}

func (s *scanner) ws() {
	for s.i < len(s.b) {
		switch s.b[s.i] {
		case ' ', '\t', '\n', '\r':
			s.i++
		default:
			return
		}
	}
}

func (s *scanner) caseArray() ([]CaseSpan, error) {
	if s.peek() != '[' {
		return nil, fmt.Errorf("cases is not an array")
	}
	s.i++
	var out []CaseSpan
	for {
		s.ws()
		if s.peek() == ']' {
			s.i++
			return out, nil
		}
		if s.peek() == ',' {
			s.i++
			continue
		}
		cs := CaseSpan{ExpectStart: -1, ExpectEnd: -1}
		last, err := s.object(func(key string) error {
			s.ws()
			start := s.i
			if err := s.skip(); err != nil {
				return err
			}
			if key == "expect" {
				cs.ExpectStart, cs.ExpectEnd = start, s.i
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		cs.LastEnd = last
		out = append(out, cs)
	}
}

// object reads an object, calling member for each key with the scanner at
// the colon's end; it returns the end of the last member's value.
func (s *scanner) object(member func(key string) error) (int, error) {
	if s.peek() != '{' {
		return 0, fmt.Errorf("not an object at %d", s.i)
	}
	s.i++
	last := s.i
	for {
		s.ws()
		switch s.peek() {
		case '}':
			s.i++
			return last, nil
		case ',':
			s.i++
			continue
		case '"':
		default:
			return 0, fmt.Errorf("bad object at %d", s.i)
		}
		key, err := s.str()
		if err != nil {
			return 0, err
		}
		s.ws()
		if s.peek() != ':' {
			return 0, fmt.Errorf("no colon at %d", s.i)
		}
		s.i++
		s.ws()
		if err := member(key); err != nil {
			return 0, err
		}
		last = s.i
	}
}

func (s *scanner) str() (string, error) {
	start := s.i
	if err := s.skip(); err != nil {
		return "", err
	}
	v, err := DecodeJSON(s.b[start:s.i])
	if err != nil {
		return "", err
	}
	k, _ := v.(string)
	return k, nil
}

// skip passes over one value.
func (s *scanner) skip() error {
	s.ws()
	switch c := s.peek(); {
	case c == '"':
		s.i++
		for s.i < len(s.b) {
			switch s.b[s.i] {
			case '\\':
				s.i += 2
				continue
			case '"':
				s.i++
				return nil
			}
			s.i++
		}
		return fmt.Errorf("unterminated string")
	case c == '{':
		_, err := s.object(func(string) error { return s.skip() })
		return err
	case c == '[':
		s.i++
		for {
			s.ws()
			switch s.peek() {
			case ']':
				s.i++
				return nil
			case ',':
				s.i++
				continue
			case 0:
				return fmt.Errorf("unterminated array")
			}
			if err := s.skip(); err != nil {
				return err
			}
		}
	default:
		start := s.i
		for s.i < len(s.b) {
			switch s.b[s.i] {
			case ',', '}', ']', ' ', '\t', '\n', '\r':
				if s.i == start {
					return fmt.Errorf("bad value at %d", s.i)
				}
				return nil
			}
			s.i++
		}
		if s.i == start {
			return fmt.Errorf("bad value at %d", s.i)
		}
		return nil
	}
}
