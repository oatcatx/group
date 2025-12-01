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
	n := &node{f: func(context.Context, any) error { return runner() }, idx: g.x, Group: g}
	g.nodes = append(g.nodes, n)
	g.x++
	return n
}

func (g *Group) AddTask(task func(context.Context) error) *node {
	n := &node{f: func(ctx context.Context, _ any) error { return task(ctx) }, idx: g.x, Group: g}
	g.nodes = append(g.nodes, n)
	g.x++
	return n
}

func (g *Group) AddSharedTask(task func(context.Context, any) error) *node {
	n := &node{f: task, idx: g.x, Group: g}
	g.nodes = append(g.nodes, n)
	g.x++
	return n
}

func (g *Group) AddNode(n Node) *node {
	node := &node{f: n.Exec, idx: g.x, Group: g}
	node.Key(n.Key()).Dep(n.Dep()...).WeakDep(n.WeakDep()...)
	g.nodes = append(g.nodes, node)
	g.x++
	return node
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

	limit := len(g.nodes) // limit defaults to the number of nodes
	if g.Limit > 0 {
		limit = g.Limit
	}

	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(limit)

	// group timeout
	if g.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, g.Timeout)
		defer cancel()
	}

	// group pre-execution interceptor
	if g.Pre != nil {
		if err = g.Pre(ctx); err != nil {
			return err
		}
	}
	var groupErrs = make([]error, len(g.nodes))
	var tracker *rollbackTracker
	var xshared any
	if len(shared) == 1 {
		xshared = shared[0]
	} else if len(shared) > 1 {
		xshared = shared
	}
	g.exec(ctx, eg, xshared, groupErrs, &tracker)
	defer func() {
		if err == nil {
			err = leafError(g.nodes, groupErrs)
		}
		// group rollback
		if err != nil && tracker != nil {
			if rbErr := tracker.rollback(ctx, xshared, groupErrs); rbErr != nil {
				err = errors.Join(err, rbErr)
			}
		}
		// group post-execution interceptor
		if g.After != nil {
			err = g.After(ctx, err)
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
				return fmt.Errorf("group %s timeout", g.Prefix)
			}
			return <-done
		case err = <-done:
			return
		}
	}
	return eg.Wait()
}

func (g *Group) exec(ctx context.Context, eg *errgroup.Group, shared any, groupErrs []error, tracker **rollbackTracker) {
	var indegree = make([]uint32, len(g.nodes))
	var rbCnt int
	for i, node := range g.nodes {
		indegree[i] = uint32(len(node.deps))
		if node.rollback != nil {
			rbCnt++
		}
	}
	if rbCnt > 0 {
		*tracker = &rollbackTracker{order: make([]*node, rbCnt)}
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
				// track for rollback
				if *tracker != nil && n.rollback != nil {
					(*tracker).track(n)
				}

				// node post-execution interceptor
				if n.after != nil {
					err = n.after(ctx, shared, err)
				}

				// wrap error and record
				ok := err == nil
				if !ok {
					groupErrs[n.idx], err = wrapError(n, err, groupErrs), nil // clear non-fast-fail error
					if n.ff {
						err = groupErrs[n.idx] // fast-fail will cancel the group context with current node's error chain
						return
					}
				}

				// notify
				if n.key != nil {
					if ok {
						for _, toIdx := range n.to {
							if atomic.AddUint32(&indegree[toIdx], ^uint32(0)) == 0 {
								run(g.nodes[toIdx])
							}
						}
					} else { // if non-fast-fail error occurs, only notify nodes that weakly depend on it
						for _, toIdx := range n.weakTo {
							if atomic.AddUint32(&indegree[toIdx], ^uint32(0)) == 0 {
								run(g.nodes[toIdx])
							}
						}
					}
				}
			}()

			if g.WithLog || g.ErrC != nil {
				defer func(start time.Time) {
					nodeMonitor(ctx, g.Prefix, n.key, start, g.WithLog, g.ErrC, err)
				}(time.Now())
			}

			execF := n.f
			if n.key != nil && store != nil {
				// wrap store func
				storeF := execF
				execF = func(ctx context.Context, shared any) error {
					return storeF(context.WithValue(ctx, storeKey{}, storeFunc(func(v any) { store.Store(n.key, v) })), shared)
				}
			}
			if n.retry > 0 {
				// wrap retry func
				retryF := execF
				execF = func(ctx context.Context, shared any) (err error) {
					for i := range n.retry + 1 {
						if err = retryF(ctx, shared); err == nil {
							break
						}
						if g.WithLog {
							slog.InfoContext(ctx, fmt.Sprintf("[Group::node -> exec] group %s: node %s retry #%d", g.Prefix, n.key, i+1))
						}
					}
					return
				}
			}
			if n.pre != nil {
				// wrap pre interceptor
				preF := execF
				execF = func(ctx context.Context, shared any) error {
					// node pre-execution interceptor
					if n.pre != nil {
						if err := n.pre(ctx, shared); err != nil {
							return err
						}
					}
					return preF(ctx, shared)
				}
			}

			if n.timeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel := context.WithTimeout(ctx, n.timeout)
				defer cancel()

				done := make(chan error, 1)
				go func() {
					done <- SafeRunNode(ctx, execF, shared)
				}()
				select {
				case <-ctx.Done():
					if errors.Is(ctx.Err(), context.DeadlineExceeded) { // actual timeout
						if g.WithLog {
							slog.InfoContext(ctx, fmt.Sprintf("[Group::node -> exec] group %s: node %s timeout", g.Prefix, n.key), slog.Duration("after", g.Timeout))
						}
						return fmt.Errorf("node %v timeout", n.key)
					}
					return <-done
				case err = <-done:
					return
				}
			}
			return SafeRunNode(ctx, execF, shared)
		})
	}

	// run root nodes
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
