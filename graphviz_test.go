package group

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	os_exec "os/exec"
	"testing"
	"time"

	"github.com/goccy/go-graphviz"
	"github.com/goccy/go-graphviz/cgraph"
	"github.com/stretchr/testify/assert"
)

func TestGraphRendering(t *testing.T) {
	t.Parallel()

	t.Run("render graph", func(t *testing.T) {
		g := NewGroup().
			AddRunner(func() error { return nil }).Key("a").
			AddRunner(func() error { return nil }).Key("b").Dep("a").Group

		var buf bytes.Buffer
		err := g.RenderGraph(context.Background(), &GraphOptions{Format: graphviz.SVG}, &buf)
		assert.Nil(t, err)
		assert.NotEmpty(t, buf.Bytes())
		assert.Contains(t, buf.String(), "svg")
	})

	t.Run("render graph image", func(t *testing.T) {
		g := NewGroup().
			AddRunner(func() error { return nil }).Key("a").
			AddRunner(func() error { return nil }).Key("b").Dep("a").
			AddRunner(func() error { return nil }).Key("c").Dep("a").
			AddRunner(func() error { return nil }).Key("d").Dep("b", "c").Group

		img, err := g.RenderGraphImage(context.Background(), nil)
		assert.Nil(t, err)
		assert.NotNil(t, img)
		assert.NoError(t, openImage(img))
	})

	t.Run("render graph to file", func(t *testing.T) {
		g := NewGroup().
			AddRunner(func() error { return nil }).Key("a").
			AddRunner(func() error { return nil }).Key("b").Dep("a").
			AddRunner(func() error { return nil }).Key("c").Dep("a").
			AddRunner(func() error { return nil }).Key("d").Dep("b", "c").Group

		tmpFile := t.TempDir() + "/test_graph.png"
		err := g.RenderGraphToFile(context.Background(), nil, tmpFile)
		assert.Nil(t, err)

		_, err = os.Stat(tmpFile)
		assert.Nil(t, err)
	})

	t.Run("DOT format output", func(t *testing.T) {
		g := NewGroup().
			AddRunner(func() error { return nil }).Key("a").
			AddRunner(func() error { return nil }).Key("b").Dep("a").
			AddRunner(func() error { return nil }).Key("c").Dep("a").Group

		dot, err := g.DOT(context.Background(), nil)
		assert.Nil(t, err)
		assert.Contains(t, dot, "a -> b")
		assert.Contains(t, dot, "a -> c")
	})

	t.Run("custom graph options", func(t *testing.T) {
		g := NewGroup().
			AddRunner(func() error { return nil }).Key("start").
			AddRunner(func() error { return nil }).Key("process").Dep("start").
			AddRunner(func() error { return nil }).Key("end").Dep("process").Group

		opts := &GraphOptions{
			Title:     "My Graph",
			Format:    graphviz.SVG,
			RankDir:   graphviz.LRRank,
			NodeShape: cgraph.EllipseShape,
			NodeColor: "#2ECC71",
			EdgeColor: "#E74C3C",
		}

		img, err := g.RenderGraphImage(context.Background(), opts)
		assert.Nil(t, err)
		assert.NotNil(t, img)
		assert.NoError(t, openImage(img))
	})
}

func TestGroupGraph(t *testing.T) {
	t.Run("basic graph generation", func(t *testing.T) {
		g := NewGroup().
			AddRunner(func() error { return nil }).Key("a").
			AddRunner(func() error { return nil }).Key("b").Dep("a").
			AddRunner(func() error { return nil }).Key("c").Dep("a").
			AddRunner(func() error { return nil }).Key("d").Dep("b", "c").Group

		img, err := g.RenderGraphImage(context.Background(), nil)
		assert.Nil(t, err)
		assert.NotNil(t, img)
		assert.NoError(t, openImage(img))
	})

	t.Run("graph with weak dependencies", func(t *testing.T) {
		g := NewGroup().
			AddRunner(func() error { return nil }).Key("a").
			AddRunner(func() error { return nil }).Key("b").Dep("a").
			AddRunner(func() error { return nil }).Key("c").WeakDep("a").
			AddRunner(func() error { return nil }).Key("d").Dep("b", "c").Group

		img, err := g.RenderGraphImage(context.Background(), nil)
		assert.Nil(t, err)
		assert.NotNil(t, img)
		assert.NoError(t, openImage(img))
	})

	t.Run("empty group", func(t *testing.T) {
		g := NewGroup()

		img, err := g.RenderGraphImage(context.Background(), nil)
		assert.Nil(t, err)
		assert.NotNil(t, img) // still produces valid empty graph
		assert.NoError(t, openImage(img))
	})

	t.Run("anonymous nodes", func(t *testing.T) {
		g := NewGroup().
			AddRunner(func() error { return nil }). // anonymous
			AddRunner(func() error { return nil }).Key("named").Group

		img, err := g.RenderGraphImage(context.Background(), nil)
		assert.Nil(t, err)
		assert.NotNil(t, img)
		assert.NoError(t, openImage(img))
	})
}

func TestGroupGraphWithAllFeatures(t *testing.T) {
	g := NewGroup(
		WithPrefix("FullFeaturedGroup"),
		WithLimit(3),
		WithPreFunc(func(context.Context) error { return nil }),
		WithAfterFunc(func(context.Context, error) error { return nil }),
		WithTimeout(30*time.Second),
		WithErrorCollector(make(chan error)),
		WithLog,
	)

	// Node A: basic node
	g.AddRunner(func() error { return nil }).Key("A")

	// Node B: with retry
	g.AddRunner(func() error { return nil }).
		Key("B").
		Dep("A").
		WithRetry(3)

	// Node C: with timeout and pre/after interceptors
	g.AddRunner(func() error { return nil }).
		Key("C").
		Dep("A").
		WithTimeout(5 * time.Second).
		WithPreFunc(func(context.Context, any) error { return nil }).
		WithAfterFunc(func(context.Context, any, error) error { return nil })

	// Node D: with rollback and fast-fail
	g.AddRunner(func() error { return nil }).
		Key("D").
		Dep("B").
		FastFail().
		WithRollback(func(context.Context, any, error) error { return nil })

	// Node E: weak dependency with all features
	g.AddRunner(func() error { return nil }).
		Key("E").
		WeakDep("C").
		WithRetry(2).
		WithTimeout(10 * time.Second).
		WithPreFunc(func(context.Context, any) error { return nil }).
		WithAfterFunc(func(context.Context, any, error) error { return nil }).
		WithRollback(func(context.Context, any, error) error { return nil })

	// Node F: final node depending on D and E, fast-fail with retry
	g.AddRunner(func() error { return nil }).
		Key("F").
		Dep("D", "E").
		FastFail().
		WithRetry(1).
		WithTimeout(15 * time.Second)

	img, err := g.RenderGraphImage(context.Background(), nil)
	assert.Nil(t, err)
	assert.NotNil(t, img)
	assert.NoError(t, openImage(img))

	// dot graphviz url
	url, err := g.GraphURL(context.Background(), nil)
	assert.Nil(t, err)
	assert.NoError(t, os_exec.Command("open", url).Start())
}

func TestComplexGraphDependencies(t *testing.T) {
	t.Run("diamond pattern with multiple layers", func(t *testing.T) {
		// Creates a complex diamond pattern:
		//        A
		//      / | \
		//     B  C  D
		//    /|\ | /|\
		//   E F G H I J
		//    \|/ \|/ |
		//     K   L  M
		//      \ | /
		//        N
		g := NewGroup(WithPrefix("DiamondPattern"))

		g.AddRunner(func() error { return nil }).Key("A")

		g.AddRunner(func() error { return nil }).Key("B").Dep("A")
		g.AddRunner(func() error { return nil }).Key("C").Dep("A")
		g.AddRunner(func() error { return nil }).Key("D").Dep("A")

		g.AddRunner(func() error { return nil }).Key("E").Dep("B")
		g.AddRunner(func() error { return nil }).Key("F").Dep("B")
		g.AddRunner(func() error { return nil }).Key("G").Dep("B", "C")
		g.AddRunner(func() error { return nil }).Key("H").Dep("C", "D")
		g.AddRunner(func() error { return nil }).Key("I").Dep("D")
		g.AddRunner(func() error { return nil }).Key("J").Dep("D")

		g.AddRunner(func() error { return nil }).Key("K").Dep("E", "F", "G")
		g.AddRunner(func() error { return nil }).Key("L").Dep("G", "H")
		g.AddRunner(func() error { return nil }).Key("M").Dep("J")

		g.AddRunner(func() error { return nil }).Key("N").Dep("K", "L", "M")

		img, err := g.RenderGraphImage(context.Background(), nil)
		assert.Nil(t, err)
		assert.NotNil(t, img)
		assert.NoError(t, openImage(img))
	})

	t.Run("mixed strong and weak dependencies web", func(t *testing.T) {
		// Complex web with mixed dependency types:
		//     A -----> B -----> C
		//     |  weak  |        |
		//     v   /    v  weak  v
		//     D <----- E -----> F
		//     |        |  weak  |
		//     v  weak  v   /    v
		//     G -----> H <----- I
		//      \       |       /
		//       \      v      /
		//        ----> J <----
		g := NewGroup(WithPrefix("MixedDependencyWeb"))

		g.AddRunner(func() error { return nil }).Key("A")
		g.AddRunner(func() error { return nil }).Key("B").Dep("A")
		g.AddRunner(func() error { return nil }).Key("C").Dep("B")

		g.AddRunner(func() error { return nil }).Key("D").Dep("A").WeakDep("B")
		g.AddRunner(func() error { return nil }).Key("E").Dep("B")
		g.AddRunner(func() error { return nil }).Key("F").Dep("C").WeakDep("E")

		g.AddRunner(func() error { return nil }).Key("G").Dep("D")
		g.AddRunner(func() error { return nil }).Key("H").Dep("E").WeakDep("F", "G")
		g.AddRunner(func() error { return nil }).Key("I").Dep("F").Node("H").Dep("I")

		g.AddRunner(func() error { return nil }).Key("J").Dep("G", "H", "I")

		img, err := g.RenderGraphImage(context.Background(), nil)
		assert.Nil(t, err)
		assert.NotNil(t, img)
		assert.NoError(t, openImage(img))
	})

	t.Run("parallel pipelines with cross dependencies", func(t *testing.T) {
		// Three parallel pipelines with cross dependencies:
		// Pipeline 1: A1 -> B1 -> C1 -> D1
		// Pipeline 2: A2 -> B2 -> C2 -> D2
		// Pipeline 3: A3 -> B3 -> C3 -> D3
		// Cross deps: B1 -> C2, B2 -> C3, C1 -> D2, C2 -> D3
		// Final: D1, D2, D3 -> E
		g := NewGroup(WithPrefix("ParallelPipelines"), WithLimit(5))

		// Pipeline 1
		g.AddRunner(func() error { return nil }).Key("A1")
		g.AddRunner(func() error { return nil }).Key("B1").Dep("A1").FastFail()
		g.AddRunner(func() error { return nil }).Key("C1").Dep("B1").WithRetry(2)
		g.AddRunner(func() error { return nil }).Key("D1").Dep("C1").WithTimeout(5 * time.Second)

		// Pipeline 2
		g.AddRunner(func() error { return nil }).Key("A2")
		g.AddRunner(func() error { return nil }).Key("B2").Dep("A2").FastFail()
		g.AddRunner(func() error { return nil }).Key("C2").Dep("B2", "B1").WithRetry(2)
		g.AddRunner(func() error { return nil }).Key("D2").Dep("C2", "C1").WithTimeout(5 * time.Second)

		// Pipeline 3
		g.AddRunner(func() error { return nil }).Key("A3")
		g.AddRunner(func() error { return nil }).Key("B3").Dep("A3").FastFail()
		g.AddRunner(func() error { return nil }).Key("C3").Dep("B3", "B2").WithRetry(2)
		g.AddRunner(func() error { return nil }).Key("D3").Dep("C3", "C2").WithTimeout(5 * time.Second)

		// Final aggregator
		g.AddRunner(func() error { return nil }).Key("E").Dep("D1", "D2", "D3").
			FastFail().WithRetry(1).
			WithRollback(func(context.Context, any, error) error { return nil })

		img, err := g.RenderGraphImage(context.Background(), nil)
		assert.Nil(t, err)
		assert.NotNil(t, img)
		assert.NoError(t, openImage(img))
	})

	t.Run("fan-out fan-in pattern", func(t *testing.T) {
		// Source fans out to many workers, then fans in to sink
		// Source -> Worker1, Worker2, ... Worker10 -> Aggregator -> Sink
		g := NewGroup(WithPrefix("FanOutFanIn"), WithLimit(20))

		g.AddRunner(func() error { return nil }).Key("Source")

		workerCount := 10
		workerKeys := make([]any, workerCount)
		for i := range workerCount {
			key := fmt.Sprintf("Worker%d", i)
			workerKeys[i] = key
			g.AddRunner(func() error { return nil }).Key(key).Dep("Source").
				WithRetry(1).WithTimeout(5 * time.Second)
		}

		g.AddRunner(func() error { return nil }).Key("Aggregator").Dep(workerKeys...).
			WithTimeout(30 * time.Second).FastFail()

		g.AddRunner(func() error { return nil }).Key("Sink").Dep("Aggregator").
			WithRollback(func(context.Context, any, error) error { return nil })

		img, err := g.RenderGraphImage(context.Background(), nil)
		assert.Nil(t, err)
		assert.NotNil(t, img)
		assert.NoError(t, openImage(img))
	})
}

func openImage(img image.Image) error {
	f, err := os.CreateTemp("", "img-*.png")
	if err != nil {
		return err
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		return err
	}
	return os_exec.Command("open", f.Name()).Start()
}
