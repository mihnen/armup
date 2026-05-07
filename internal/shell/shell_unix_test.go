//go:build linux || darwin

package shell

import (
	"strings"
	"testing"
)

// stripBlock cuts the marker block out of an rc file body. The shape of
// the block matches what EnsureOnPath writes:
//
//	\n
//	# >>> armup >>>
//	export PATH="..."
//	# <<< armup <<<
//	\n
func TestStripBlock(t *testing.T) {
	rc := `# .zshrc
plugins=(git fzf-tab)
source $ZSH/oh-my-zsh.sh

# >>> armup >>>
export PATH="/home/u/.local/share/arm-toolchains/current/bin:$PATH"
# <<< armup <<<

alias ll='ls -la'
`
	got, removed := stripBlock(rc)
	if !removed {
		t.Fatal("stripBlock should have reported a removal")
	}
	if strings.Contains(got, "armup") {
		t.Errorf("residual 'armup' marker after stripBlock:\n%s", got)
	}
	if !strings.Contains(got, "plugins=(git fzf-tab)") {
		t.Error("removed too much — earlier rc content went away")
	}
	if !strings.Contains(got, "alias ll='ls -la'") {
		t.Error("removed too much — later rc content went away")
	}
}

func TestStripBlockNoMarkers(t *testing.T) {
	rc := "# .zshrc\nexport FOO=bar\n"
	got, removed := stripBlock(rc)
	if removed {
		t.Error("stripBlock reported removal when no markers present")
	}
	if got != rc {
		t.Errorf("stripBlock modified content with no markers:\ngot:  %q\nwant: %q", got, rc)
	}
}

func TestStripBlockMalformedNoEnd(t *testing.T) {
	// BeginMarker present but no EndMarker — leave the file alone rather than
	// truncating to EOF.
	rc := "# >>> armup >>>\nexport PATH=...\n"
	got, removed := stripBlock(rc)
	if removed {
		t.Error("stripBlock should not modify content with unterminated block")
	}
	if got != rc {
		t.Errorf("content changed despite malformed block")
	}
}
