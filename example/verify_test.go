package group

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/oatcatx/group"
)

// region VERIFY

func TestGroupGoMissingDep(t *testing.T) {
	t.Parallel()

	assert.PanicsWithValue(t, fmt.Sprintf("missing dependency %q -> %q", "b", "x"), func() {
		var f, x = func() error { return nil }, func() error { return nil }
		NewGroup().
			AddRunner(f).Key("a").
			AddRunner(x).Key("b").Dep("x").
			Verify(true)
	})
}

func TestGroupGoVerify(t *testing.T) {
	t.Parallel()

	defer func() {
		if x := recover(); x != nil {
			// range over map is random
			assert.Contains(t, []string{
				`dependency cycle detected: "a" -> "c" -> "b" -> "a"`,
				`dependency cycle detected: "b" -> "a" -> "c" -> "b"`,
				`dependency cycle detected: "c" -> "b" -> "a" -> "c"`,
			}, x)
		}
	}()

	var a, b, c = func() error { return nil }, func() error { return nil }, func() error { return nil }
	g := NewGroup().
		AddRunner(a).Key("a").
		AddRunner(b).Key("b").Dep("a").
		AddRunner(c).Key("c").Dep("b").Group
	g.Node("a").Dep("c").Verify(true)
}
