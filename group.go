package group

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
)

type Group struct {
	x      int
	nodes  []*node
	idxMap map[any]int
	Options
}

func NewGroup(opts ...option) *Group {
	g := &Group{
		nodes:   make([]*node, 0),
		idxMap:  make(map[any]int),
		Options: *Opts(opts...),
	}
	if g.Prefix == "" {
		g.Prefix = "anonymous" // default prefix
	}
	return g
}

func (g *Group) AddRunner(runner func() error) *node {
	n := &node{f: runner, idx: g.x, Group: g}
	g.nodes = append(g.nodes, n)
	g.x++
	return n
}

func (g *Group) AddTask(task func(context.Context) error) *node {
	n := &node{fc: task, idx: g.x, Group: g}
	g.nodes = append(g.nodes, n)
	g.x++
	return n
}

func (g *Group) AddSharedTask(task func(context.Context, any) error) *node {
	n := &node{fca: task, idx: g.x, Group: g}
	g.nodes = append(g.nodes, n)
	g.x++
	return n
}

func (g *Group) Node(key any) *node {
	if idx, ok := g.idxMap[key]; ok {
		return g.nodes[idx]
	}
	return nil
}

// if shared units are provided, they will be passed to the shared tasks
// if len(shared) == 1, the task receives shared[0] (type any)
// if len(shared) > 1, the task receives shared (type []any)
// multiple shared units are not recommended
func (g *Group) Go(ctx context.Context, shared ...any) (err error) {
	if len(g.nodes) == 0 {
		return nil
	}

	if g.WithLog {
		defer func(start time.Time) {
			groupMonitor(ctx, "Group.Go", g.Prefix, start, g.WithLog, err)
		}(time.Now())
	}

	eg, ctx := errgroup.WithContext(ctx)
	limit := len(g.nodes) // limit defaults to the number of nodes
	if g.Limit > 0 {
		limit = g.Limit
	}
	eg.SetLimit(limit)
	// group timeout
	if g.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, g.Timeout)
		defer cancel()
	}

	groupErrs := make([]error, len(g.nodes))
	var xshared any
	if len(shared) == 1 {
		xshared = shared[0]
	} else if len(shared) > 1 {
		xshared = shared
	}
	g.exec(ctx, eg, groupErrs, xshared)
	defer func() {
		if err == nil {
			err = leafError(g.nodes, groupErrs)
		}
	}()

	// outer timeout control
	if g.Timeout > 0 {
		done := make(chan error, 1)
		go func() {
			done <- eg.Wait()
		}()
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) { // actual timeout
				if g.WithLog {
					slog.InfoContext(ctx, fmt.Sprintf("[Group::Group.Go] group %s timeout", g.Prefix), slog.Duration("after", g.Timeout))
				}
				return errors.New("group timeout")
			}
			return <-done
		case err := <-done:
			return err
		}
	}
	return eg.Wait()
}

func (g *Group) exec(ctx context.Context, eg *errgroup.Group, groupErrs []error, shared any) {
	var indegree = make([]uint32, len(g.nodes))
	for i, node := range g.nodes {
		indegree[i] = uint32(len(node.deps))
	}
	store, _ := ctx.Value(fetchKey{}).(Storer)
	var run func(node *node)
	run = func(n *node) {
		eg.Go(func() (err error) {
			select {
			case <-ctx.Done(): // ctx check
				return ctx.Err() // fast-fail triggered or ctx timeout
			default: // ctx ok
			}

			defer func() {
				ok := err == nil
				if !ok {
					groupErrs[n.idx] = wrapError(n, err, groupErrs)
					// fast-fail will cancel the group context with current node's error chain
					if n.ff {
						err = groupErrs[n.idx]
						return
					}
					err = nil // clear non-fast-fail error
				}

				if n.key == nil {
					return
				}
				// notify
				notify := func(idx int) {
					if atomic.AddUint32(&indegree[idx], ^uint32(0)) == 0 {
						run(g.nodes[idx])
					}
				}
				if ok {
					for _, toIdx := range n.to {
						notify(toIdx)
					}
				} else { // if non-fast-fail error occurs, only notify nodes that weakly depend on it
					for _, toIdx := range n.weakTo {
						notify(toIdx)
					}
				}
			}()

			if g.WithLog || g.ErrC != nil {
				defer func(start time.Time) {
					var name string
					switch {
					case n.f != nil:
						name = funcName(n.f)
					case n.fc != nil:
						name = funcName(n.fc)
					case n.fca != nil:
						name = funcName(n.fca)
					}
					funcMonitor(ctx, "Group.Go —> exec", g.Prefix, name, start, g.WithLog, g.ErrC, err)
				}(time.Now())
			}

			var f = func(ctx context.Context) error {
				switch {
				case n.f != nil:
					return SafeRun(ctx, n.f)
				case n.fc != nil:
					return SafeRunCtx(ctx, n.fc)
				case n.fca != nil:
					return SafeRunCtxShared(ctx, n.fca, shared)
				}
				return nil
			}
			if n.key != nil && store != nil {
				// wrap store func
				return f(context.WithValue(ctx, storeKey{}, storeFunc(func(v any) { store.Store(n.key, v) })))
			}
			return f(ctx)
		})
	}
	for _, node := range g.nodes {
		if len(node.deps) == 0 {
			run(node)
		}
	}
}

func (g *Group) Verify(panicking bool) string {
	if len(g.nodes) == 0 {
		return ""
	}
	type token = struct{}
	graph, src := make(map[any][]any, len(g.nodes)), make(map[any]token, len(g.nodes))
	for _, node := range g.nodes {
		if node == nil {
			continue
		}
		key := node.key
		// skip anonymous runner
		if key == nil {
			continue
		}
		graph[key], src[key] = make([]any, 0, len(node.deps)), token{}
		for _, depIdx := range node.deps {
			if depKey := g.nodes[depIdx].key; depKey != nil {
				graph[key] = append(graph[key], depKey)
			}
		}
	}

	// check cycle dfs
	stk, visited := make(map[any]token), make(map[any]token)
	var dfs func(key any) any
	dfs = func(key any) any {
		if _, ok := stk[key]; ok {
			return fmt.Sprintf("%q", key)
		}
		if _, ok := visited[key]; ok {
			return ""
		}
		stk[key], visited[key] = token{}, token{}
		for _, dep := range graph[key] {
			if x := dfs(dep); x != "" {
				return fmt.Sprintf("%q -> %s", key, x)
			}
		}
		delete(stk, key)
		return ""
	}
	// check cycle
	for key := range graph {
		if _, ok := visited[key]; !ok {
			if x := dfs(key); x != "" {
				var msg = fmt.Sprintf("dependency cycle detected: %s", x)
				if panicking {
					panic(msg)
				}
				return msg
			}
		}
	}
	return ""
}
