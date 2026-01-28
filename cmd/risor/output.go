package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/deepnoodle-ai/wonton/cli"
	"github.com/deepnoodle-ai/wonton/color"
)

func formatOutput(ctx *cli.Context, result any) (string, error) {
	format := strings.ToLower(ctx.String("output"))
	noColor := ctx.Bool("no-color")

	switch format {
	case "":
		// Default: try JSON, fall back to string representation
		if result == nil {
			return "", nil
		}
		output, err := formatJSON(result, noColor)
		if err != nil {
			return fmt.Sprintf("%v", result), nil
		}
		return string(output), nil
	case "json":
		output, err := formatJSON(result, noColor)
		if err != nil {
			return "", err
		}
		return string(output), nil
	case "text":
		return fmt.Sprintf("%v", result), nil
	default:
		return "", fmt.Errorf("unknown output format: %s", format)
	}
}

func formatJSON(result any, noColor bool) ([]byte, error) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, err
	}

	// Apply colors if enabled
	if !noColor && color.ShouldColorize(os.Stdout) {
		data = colorizeJSON(data)
	}

	return data, nil
}

// colorizeJSON applies syntax highlighting to JSON output
func colorizeJSON(data []byte) []byte {
	s := string(data)
	var result strings.Builder
	result.Grow(len(s) * 2) // Estimate for ANSI codes

	inString := false
	escapeNext := false
	isKey := true

	for i := 0; i < len(s); i++ {
		c := s[i]

		if escapeNext {
			result.WriteByte(c)
			escapeNext = false
			continue
		}

		if c == '\\' && inString {
			escapeNext = true
			result.WriteByte(c)
			continue
		}

		if c == '"' {
			if inString {
				result.WriteByte(c)
				result.WriteString("\033[0m")
				inString = false
			} else {
				inString = true
				if isKey {
					result.WriteString(color.Cyan.ForegroundSeq())
				} else {
					result.WriteString(color.Green.ForegroundSeq())
				}
				result.WriteByte(c)
			}
			continue
		}

		if !inString {
			switch c {
			case ':':
				isKey = false
				result.WriteByte(c)
			case ',', '\n':
				if c == '\n' || (c == ',' && i+1 < len(s) && s[i+1] == '\n') {
					isKey = true
				}
				result.WriteByte(c)
			case '{', '}', '[', ']':
				result.WriteString(color.White.ForegroundSeq())
				result.WriteByte(c)
				result.WriteString("\033[0m")
			case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '-', '.':
				// Numbers
				result.WriteString(color.Yellow.ForegroundSeq())
				result.WriteByte(c)
				// Continue reading the number
				for i+1 < len(s) {
					next := s[i+1]
					if (next >= '0' && next <= '9') || next == '.' || next == 'e' || next == 'E' || next == '+' || next == '-' {
						i++
						result.WriteByte(next)
					} else {
						break
					}
				}
				result.WriteString("\033[0m")
			default:
				// Check for true/false/null
				if i+4 <= len(s) && s[i:i+4] == "true" {
					result.WriteString(color.Magenta.ForegroundSeq())
					result.WriteString("true")
					result.WriteString("\033[0m")
					i += 3
				} else if i+5 <= len(s) && s[i:i+5] == "false" {
					result.WriteString(color.Magenta.ForegroundSeq())
					result.WriteString("false")
					result.WriteString("\033[0m")
					i += 4
				} else if i+4 <= len(s) && s[i:i+4] == "null" {
					result.WriteString(color.BrightBlack.ForegroundSeq())
					result.WriteString("null")
					result.WriteString("\033[0m")
					i += 3
				} else {
					result.WriteByte(c)
				}
			}
		} else {
			result.WriteByte(c)
		}
	}

	return []byte(result.String())
}
