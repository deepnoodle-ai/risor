package tmpl

import (
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

func TestParseString(t *testing.T) {
	tests := []struct {
		input string
		want  []*Fragment
	}{
		{
			"Hello ${name}!",
			[]*Fragment{
				{value: "Hello ", isVariable: false, offset: 0},
				{value: "name", isVariable: true, offset: 8},
				{value: "!", isVariable: false, offset: 13},
			},
		},
		{
			"ab ${foo} $bar baz\t",
			[]*Fragment{
				{value: "ab ", isVariable: false, offset: 0},
				{value: "foo", isVariable: true, offset: 5},
				{value: " $bar baz\t", isVariable: false, offset: 9},
			},
		},
		{
			"${ hi + 3 }${h[0]+foo.bar()}X${}",
			[]*Fragment{
				{value: " hi + 3 ", isVariable: true, offset: 2},
				{value: "h[0]+foo.bar()", isVariable: true, offset: 13},
				{value: "X", isVariable: false, offset: 28},
				{value: "", isVariable: true, offset: 31},
			},
		},
		{
			`plain text without interpolation`,
			[]*Fragment{
				{value: "plain text without interpolation", isVariable: false, offset: 0},
			},
		},
		{
			`{not interpolation}`,
			[]*Fragment{
				{value: "{not interpolation}", isVariable: false, offset: 0},
			},
		},
		{
			"é${x}",
			[]*Fragment{
				{value: "é", isVariable: false, offset: 0},
				{value: "x", isVariable: true, offset: 3},
			},
		},
	}
	for _, tc := range tests {
		res, err := Parse(tc.input)
		assert.Nil(t, err)
		assert.Equal(t, res.Value(), tc.input)
		assert.Equal(t, res.Fragments(), tc.want)
	}
}

// An expression ends at the '}' that matches its "${". Nested braces and
// braces inside string literals do not end it.
func TestParseNestedBraces(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`${ {a: 1}["a"] }`, ` {a: 1}["a"] `},
		{`${ if (true) { "yes" } else { "no" } }`, ` if (true) { "yes" } else { "no" } `},
		{`${ "}" }`, ` "}" `},
		{`${ '}' }`, ` '}' `},
		{`${ "{" }`, ` "{" `},
		{`${ "\"}" }`, ` "\"}" `},
		{`${ '\'}' }`, ` '\'}' `},
		{`${ "it's" }`, ` "it's" `},
		{`${ [1].map(x => { return {v: x} }) }`, ` [1].map(x => { return {v: x} }) `},
		// Comments: quotes and braces in them are ignored
		{`${ 1 // it's }`, ` 1 // it's `},
		{`${ 1 // { }`, ` 1 // { `},
		{"${ 1 // it's {\n }", " 1 // it's {\n "},
		{`${ 1 /* { ' */ }`, ` 1 /* { ' */ `},
		{`${ 1 /* } */ }`, ` 1 /* } */ `},
		{`${ 4 / 2 }`, ` 4 / 2 `},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			res, err := Parse(tc.input + "!")
			assert.Nil(t, err)
			frags := res.Fragments()
			assert.Len(t, frags, 2)
			assert.Equal(t, frags[0].Value(), tc.want)
			assert.True(t, frags[0].IsVariable())
			assert.Equal(t, frags[1].Value(), "!")
		})
	}
}

func TestParseStringErrors(t *testing.T) {
	tests := []struct {
		input   string
		wantErr string
	}{
		{"${", `missing '}' in template: ${`},
		{"a${0} ${cd", `missing '}' in template: a${0} ${cd`},
		{"${ {a: 1 }", `missing '}' in template: ${ {a: 1 }`},
		{`${ "} }`, `missing '}' in template: ${ "} }`},
		{`${ 1 /* } }`, `missing '}' in template: ${ 1 /* } }`},
	}
	for _, tc := range tests {
		_, err := Parse(tc.input)
		assert.NotNil(t, err)
		assert.Equal(t, err.Error(), tc.wantErr)
	}
}
