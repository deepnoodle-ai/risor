package runner

import (
	"testing"

	"github.com/deepnoodle-ai/wonton/assert"
)

func TestUntilFiles(t *testing.T) {
	// A file of a package this implementation offers is not run; of any
	// other, it is.
	assert.True(t, Skipped("suite/core-run/until-f04-as-is.json"))
	assert.True(t, Skipped("x/until-f07-methods.json"))
	assert.True(t, Skipped("x/until-f05-tagged-unions.json"))
	assert.True(t, Skipped("x/until-f13-math-functions.json"))
	assert.False(t, Skipped("x/until-f14-later.json"))
	assert.True(t, Skipped("x/until-f08-parse.json"))
	assert.False(t, Skipped("x/unions-until-texts.json"))
	assert.False(t, Skipped("x/until-fx-bad.json"))
	old := Offered["F8"]
	Offered["F8"] = true
	assert.True(t, Skipped("x/until-f08-parse.json"))
	Offered["F8"] = old
}

func TestCheckOnlyFiles(t *testing.T) {
	assert.True(t, CheckOnlyFile("suite/strings/strings-check.json"))
	assert.True(t, CheckOnlyFile("suite/queries/queries-check.json"))
	assert.False(t, CheckOnlyFile("suite/core-check/check-library.json"))
	assert.False(t, CheckOnlyFile("suite/queries/queries-run.json"))
}
