package group

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"io"
	"net/url"
	"os"
	"slices"
	"strings"

	"github.com/goccy/go-graphviz"
	"github.com/goccy/go-graphviz/cgraph"
)

type GraphOptions struct {
	Title         string           // graph title
	Format        graphviz.Format  // output format
	RankDir       cgraph.RankDir   // direction: TB (top-bottom), LR (left-right), BT, RL
	NodeShape     cgraph.Shape     // node shape: box, ellipse, circle, etc.
	NodeColor     string           // node color
	FastFailColor string           // color for fast-fail nodes
	EdgeColor     string           // edge color
	WeakEdgeStyle cgraph.EdgeStyle // style for weak dependency edges (dashed, dotted)
	ShowGroupInfo bool             // show group options in title
	ShowNodeSpec  bool             // show node spec details
}

func DefaultGraphOptions() *GraphOptions {
	return &GraphOptions{
		Format:        graphviz.PNG,
		RankDir:       cgraph.TBRank,
		NodeShape:     cgraph.BoxShape,
		NodeColor:     "#7FFFD4",
		FastFailColor: "#D2042D",
		EdgeColor:     "#333333",
		WeakEdgeStyle: cgraph.DashedEdgeStyle,
		ShowGroupInfo: true,
		ShowNodeSpec:  true,
	}
}

func (g *Group) RenderGraph(ctx context.Context, opts *GraphOptions, w io.Writer) error {
	if opts == nil {
		opts = DefaultGraphOptions()
	}

	gv, err := graphviz.New(ctx)
	if err != nil {
		return fmt.Errorf("failed to create graphviz: %w", err)
	}
	defer gv.Close()

	graph, err := gv.Graph()
	if err != nil {
		return fmt.Errorf("failed to create graph: %w", err)
	}
	defer graph.Close()

	// Set graph attributes
	if opts.Title != "" {
		graph.SetLabel(opts.Title)
	} else if opts.ShowGroupInfo {
		graph.SetLabel(buildGraphTitle(g))
	}
	graph.SetLabelLocation(cgraph.TopLocation)
	graph.SetRankDir(opts.RankDir)

	// Create nodes
	nodeMap := make(map[int]*cgraph.Node)
	for _, n := range g.nodes {
		node, err := graph.CreateNodeByName(nodeName(n))
		if err != nil {
			return fmt.Errorf("failed to create node %v: %w", n.key, err)
		}
		node.SetShape(cgraph.Shape(opts.NodeShape))
		node.SetStyle(cgraph.FilledNodeStyle)
		node.SetFontColor("black")
		if n.ff {
			node.SetFillColor(opts.FastFailColor)
		} else {
			node.SetFillColor(opts.NodeColor)
		}
		if opts.ShowNodeSpec {
			node.SetLabel(buildNodeLabel(n))
		}
		nodeMap[n.idx] = node
	}
	// Create edges
	for _, n := range g.nodes {
		toNode := nodeMap[n.idx]
		for _, depIdx := range n.deps {
			fromNode := nodeMap[depIdx]
			edge, err := graph.CreateEdgeByName("", fromNode, toNode)
			if err != nil {
				return fmt.Errorf("failed to create edge: %w", err)
			}
			edge.SetColor(opts.EdgeColor)
			// weak dep
			if slices.Contains(g.nodes[depIdx].weakTo, n.idx) {
				edge.SetStyle(opts.WeakEdgeStyle)
				edge.SetLabel("weak")
				edge.SetFontSize(10)
			} else {
				edge.SetLabel("") // empty label for strong edges
			}
		}
	}
	if err := gv.Render(ctx, graph, opts.Format, w); err != nil {
		return fmt.Errorf("failed to render graph: %w", err)
	}
	return nil
}

func (g *Group) RenderGraphImage(ctx context.Context, opts *GraphOptions) (image.Image, error) {
	if opts != nil {
		opts.Format = graphviz.PNG
	}

	var buf bytes.Buffer
	if err := g.RenderGraph(ctx, opts, &buf); err != nil {
		return nil, err
	}
	img, _, err := image.Decode(&buf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}
	return img, nil
}

func (g *Group) RenderGraphToFile(ctx context.Context, opts *GraphOptions, filename string) error {
	if opts == nil {
		opts = DefaultGraphOptions()
		// Infer format from filename extension
		opts.Format = inferFormat(filename)
	}
	var buf bytes.Buffer
	if err := g.RenderGraph(ctx, opts, &buf); err != nil {
		return err
	}
	return os.WriteFile(filename, buf.Bytes(), 0644)
}

// dot format graph
func (g *Group) DOT(ctx context.Context) (string, error) {
	opts := DefaultGraphOptions()
	opts.Format = graphviz.XDOT
	var buf bytes.Buffer
	if err := g.RenderGraph(ctx, opts, &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// dot graphviz url
func (g *Group) GraphUrl(ctx context.Context) (string, error) {
	dot, err := g.DOT(ctx)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("https://dreampuf.github.io/GraphvizOnline/#%s", url.PathEscape(dot)), nil
}

func buildGraphTitle(g *Group) string {
	var title = fmt.Sprintf("Group: %s", g.Prefix)
	var infoParts []string
	if g.Limit > 0 {
		infoParts = append(infoParts, fmt.Sprintf("limit=%d", g.Limit))
	}
	if g.Pre != nil {
		infoParts = append(infoParts, "pre=✓")
	}
	if g.After != nil {
		infoParts = append(infoParts, "after=✓")
	}
	if g.Timeout > 0 {
		infoParts = append(infoParts, fmt.Sprintf("timeout=%s", g.Timeout))
	}
	if g.ErrC != nil {
		infoParts = append(infoParts, "errC=✓")
	}
	if g.WithLog {
		infoParts = append(infoParts, "log=✓")
	}
	if len(infoParts) > 0 {
		return fmt.Sprintf("%s\\n[%s]", title, strings.Join(infoParts, " | "))
	}
	return title
}

func buildNodeLabel(n *node) string {
	var name = nodeName(n)
	var details []string
	if n.ff {
		details = append(details, "⚡ fast-fail")
	}
	if n.retry > 0 {
		details = append(details, fmt.Sprintf("🔄 retry=%d", n.retry))
	}
	if n.pre != nil {
		details = append(details, "▶ pre")
	}
	if n.after != nil {
		details = append(details, "◀ after")
	}
	if n.rollback != nil {
		details = append(details, "↩ rollback")
	}
	if n.timeout > 0 {
		details = append(details, fmt.Sprintf("⏱ timeout=%s", n.timeout))
	}
	if len(details) == 0 {
		return name
	}
	return fmt.Sprintf("%s\\n─────\\n%s", name, strings.Join(details, "\\n"))
}

func nodeName(n *node) string {
	if n.key != nil {
		return fmt.Sprintf("%v", n.key)
	}
	return fmt.Sprintf("node_%d", n.idx)
}

func inferFormat(filename string) graphviz.Format {
	if len(filename) < 4 {
		return graphviz.PNG
	}
	ext := filename[len(filename)-4:]
	switch ext {
	case ".dot":
		return graphviz.XDOT
	case ".svg":
		return graphviz.SVG
	case ".jpg":
		return graphviz.JPG
	default:
		return graphviz.PNG
	}
}
