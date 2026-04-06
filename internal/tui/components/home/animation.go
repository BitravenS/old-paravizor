package home

// animation.go — hot-swappable 64×16 ASCII cat animation.
// To replace: edit catFrames below. Each frame must be ≤64 cols × 16 rows; Render pads to 64.

import (
	"image/color"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/bitravens/paravizor/v1/internal/theme"
)

// AnimWidth / AnimHeight are the fixed canvas dimensions.
const (
	AnimWidth    = 64
	AnimHeight   = 16
	AnimInterval = 150 * time.Millisecond
)

// Each string in catFrames is a 32×16 block joined by '\n'.
var catFrames = []string{
	strings.Join([]string{
		`                                `,
		`   /\_____/\                    `,
		`  /  o   o  \                   `,
		` ( ==  ^  == )                  `,
		`  )         (                   `,
		` (           )                  `,
		`( (  )   (  ) )                 `,
		`(__(__)___(__)__)                `,
		`                                `,
		`  -~*~-~*~-~*~-~*~-~*~-~*~-   `,
		`                                `,
		`   |\      _,,,---,,_           `,
		`   ZZzz...                      `,
		`  |,4-  ) )-,_..               `,
		`  ~---~(_/--~  ~_)             `,
		`                                `,
	}, "\n"),
	strings.Join([]string{
		`                                `,
		`   /\_____/\                    `,
		`  /  -   -  \                   `,
		` ( ==  ^  == )                  `,
		`  )  ~~~~~  (                   `,
		` (           )                  `,
		`( (  )   (  ) )                 `,
		`(__(__)___(__)__)                `,
		`                                `,
		`  ~-*~-~*~-~*~-~*~-~*~-~*~-~  `,
		`                                `,
		`   |\      _,,,---,,_           `,
		`    Zzzz...                     `,
		`  |,4-  ) )-,_..               `,
		`  ~---~(_/--~  ~_)             `,
		`                                `,
	}, "\n"),
	strings.Join([]string{
		`                                `,
		`   /\_____/\                    `,
		`  /  ^   ^  \                   `,
		` ( ==  w  == )                  `,
		`  )         (                   `,
		` (    ~~~    )                  `,
		`( (  )   (  ) )                 `,
		`(__(__)___(__)__)                `,
		`                                `,
		`  -~*~-~*~-~*~-~*~-~*~-~*~-   `,
		`                                `,
		`      |\      _,,,---,,_        `,
		`       zzZ...                   `,
		`     |,4-  ) )-,_..            `,
		`     ~---~(_/--~  ~_)          `,
		`                                `,
	}, "\n"),
	strings.Join([]string{
		`                                `,
		`   /\_____/\                    `,
		`  /  o   o  \                   `,
		` ( ==  ^  == )                  `,
		`  )  ~~~~~  (                   `,
		` (           )                  `,
		`( (  )   (  ) )                 `,
		`(__(__)___(__)__)                `,
		`                                `,
		`  ~-*~-~*~-~*~-~*~-~*~-~*~-~  `,
		`                                `,
		`      |\      _,,,---,,_        `,
		`        Zz...                   `,
		`     |,4-  ) )-,_..            `,
		`     ~---~(_/--~  ~_)          `,
		`                                `,
	}, "\n"),
}

// CatAnimation holds tick state for the animation.
type CatAnimation struct {
	tick int
}

// Tick advances the animation by one step.
func (a *CatAnimation) Tick() { a.tick++ }

// Render returns the current frame as a styled string using theme colors.
// Output is always AnimWidth × AnimHeight characters.
func (a *CatAnimation) Render(th *theme.Theme) string {
	frame := catFrames[a.tick%len(catFrames)]

	// Cycle through theme colors for a lively effect.
	palette := []color.Color{
		th.WarningText,
		th.SuccessText,
		th.PrimaryText,
		th.SecondaryText,
	}
	col := palette[a.tick%len(palette)]

	lines := strings.Split(frame, "\n")
	for len(lines) < AnimHeight {
		lines = append(lines, strings.Repeat(" ", AnimWidth))
	}
	lines = lines[:AnimHeight]

	var out []string
	for _, l := range lines {
		runes := []rune(l)
		for len(runes) < AnimWidth {
			runes = append(runes, ' ')
		}
		out = append(out, lipgloss.NewStyle().Foreground(col).Render(string(runes[:AnimWidth])))
	}
	return strings.Join(out, "\n")
}
