package aquarium

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/ttbakiatwoam/asciiquarium-stream/internal/engine"
)

// art.json is extracted verbatim from the original Perl asciiquarium by
// tools/extract-art. The ASCII art is by Joan Stark and the other
// asciiquarium contributors and is distributed under the GPL-2.0-or-later.
//
//go:embed art/art.json
var artJSON []byte

type stringOrList []string

func (s *stringOrList) UnmarshalJSON(b []byte) error {
	var one string
	if err := json.Unmarshal(b, &one); err == nil {
		*s = []string{one}
		return nil
	}
	var many []string
	if err := json.Unmarshal(b, &many); err != nil {
		return err
	}
	*s = many
	return nil
}

type artEntity struct {
	Shape        stringOrList `json:"shape"`
	Color        stringOrList `json:"color"`
	DefaultColor string       `json:"default_color"`
}

type fishArt struct {
	Shape string `json:"shape"`
	Color string `json:"color"`
}

type artBook struct {
	newFish []fishArt
	oldFish []fishArt
	// objects maps creature name -> direction ("0" = left to right,
	// "1" = right to left) -> entities in the order the Perl code created
	// them.
	objects map[string]map[string][]artEntity
}

var art = mustLoadArt()

func mustLoadArt() *artBook {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(artJSON, &raw); err != nil {
		panic(err)
	}
	b := &artBook{objects: map[string]map[string][]artEntity{}}
	for k, v := range raw {
		var err error
		switch k {
		case "new_fish":
			err = json.Unmarshal(v, &b.newFish)
		case "old_fish":
			err = json.Unmarshal(v, &b.oldFish)
		default:
			var m map[string][]artEntity
			err = json.Unmarshal(v, &m)
			b.objects[k] = m
		}
		if err != nil {
			panic(fmt.Sprintf("art %s: %v", k, err))
		}
	}
	return b
}

// get returns the idx'th entity of a creature for a direction (0 or 1).
func (b *artBook) get(name string, dir, idx int) artEntity {
	return b.objects[name][fmt.Sprint(dir)][idx]
}

// sprites builds the frames for an art entity.
func (ae artEntity) sprites(autoTrans bool, def engine.Color) []*engine.Sprite {
	if ae.DefaultColor != "" {
		if c, ok := engine.ParseColor(ae.DefaultColor); ok {
			def = c
		}
	}
	return engine.NewSprites(ae.Shape, ae.Color, engine.SpriteOpts{Default: def, AutoTrans: autoTrans})
}
