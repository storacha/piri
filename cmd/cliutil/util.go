package cliutil

import (
	"fmt"
	"io"

	"github.com/labstack/gommon/color"
	"github.com/storacha/go-ucanto/did"

	"github.com/storacha/piri/pkg/build"
)

func PrintHero(w io.Writer, id did.DID) {
	fmt.Fprintf(w, `
▗▄▄▖ ▄  ▄▄▄ ▄  %s
▐▌ ▐▌▄ █    ▄  %s
▐▛▀▘ █ █    █  %s
▐▌   █      █  %s

🔥 %s
🆔 %s
🚀 Ready!
`,
		color.Green(" ▗"),
		color.Red(" █")+color.Red("▌", color.D),
		color.Red("▗", color.B)+color.Red("█")+color.Red("▘", color.D),
		color.Red("▀")+color.Red("▘", color.D),
		build.Version, id.String())
}
