package gen

import (
	"fmt"
	"os"
	"testing"

	"github.com/deepnoodle-ai/risor/fuzz/internal/types"
)

// Every value set of a soundness program conforms to its declared types.
func TestSoundValuesConform(t *testing.T) {
	lib := loadLib(t)
	for i := range 500 {
		p := GenerateSound(lib, Options{}, 5, 1, i)
		for _, c := range p.Cases {
			for _, v := range c.Vars {
				ty, err := types.Parse(v.Type)
				if err != nil {
					t.Fatalf("%s: %v", v.Type, err)
				}
				if !types.Conforms(v.Value, ty) {
					t.Fatalf("%s: %s does not conform to %s", c.Name, jsonValue(v.Value), v.Type)
				}
			}
		}
		if os.Getenv("SOUNDDEBUG") != "" && i < 10 {
			fmt.Println(p.Careful, string(p.Cases[0].JSON()))
		}
	}
}
