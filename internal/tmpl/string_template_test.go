package tmpl

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseString(t *testing.T) {
	tests := []struct {
		input string
		want  []*Fragment
	}{
		{
			"Hello ${name}!",
			[]*Fragment{
				{value: "Hello ", isVariable: false},
				{value: "name", isVariable: true},
				{value: "!", isVariable: false},
			},
		},
		{
			"ab ${foo} $bar baz\t",
			[]*Fragment{
				{value: "ab ", isVariable: false},
				{value: "foo", isVariable: true},
				{value: " $bar baz\t", isVariable: false},
			},
		},
		{
			"${ hi + 3 }${h[0]+foo.bar()}X${}",
			[]*Fragment{
				{value: " hi + 3 ", isVariable: true},
				{value: "h[0]+foo.bar()", isVariable: true},
				{value: "X", isVariable: false},
				{value: "", isVariable: true},
			},
		},
		{
			`plain text without interpolation`,
			[]*Fragment{
				{value: "plain text without interpolation", isVariable: false},
			},
		},
		{
			`{not interpolation}`,
			[]*Fragment{
				{value: "{not interpolation}", isVariable: false},
			},
		},
	}
	for _, tc := range tests {
		res, err := Parse(tc.input)
		require.Nil(t, err)
		require.Equal(t, tc.input, res.Value())
		require.Equal(t, tc.want, res.Fragments())
	}
}

func TestParseStringErrors(t *testing.T) {
	tests := []struct {
		input   string
		wantErr string
	}{
		{"${", `missing '}' in template: ${`},
		{"a${0} ${cd", `missing '}' in template: a${0} ${cd`},
	}
	for _, tc := range tests {
		_, err := Parse(tc.input)
		require.NotNil(t, err)
		require.Equal(t, tc.wantErr, err.Error())
	}
}
