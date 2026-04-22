package group

import (
	"context"
	"fmt"
)

func (g *Group) addNode(f func(context.Context, any) error) *node {
	n := &node{f: f, idx: g.x, Group: g}
	g.nodes = append(g.nodes, n)
	g.x++
	return n
}

func autoWrapper(autoF func(context.Context, any) (any, error)) func(context.Context, any) error {
	return func(ctx context.Context, shared any) error {
		v, err := autoF(ctx, shared)
		if err == nil {
			Store(ctx, v)
		}
		return err
	}
}

// region Adding Operations

func (g *Group) AddRunner(runner func() error) *node {
	return g.addNode(func(context.Context, any) error { return runner() })
}

func (g *Group) AddSharedRunner(runner func(any) error) *node {
	return g.addNode(func(_ context.Context, shared any) error { return runner(shared) })
}

func (g *Group) AddAutoRunner(runner func() (any, error)) *node {
	return g.addNode(autoWrapper(func(context.Context, any) (any, error) { return runner() }))
}

func (g *Group) AddAutoSharedRunner(runner func(any) (any, error)) *node {
	return g.addNode(autoWrapper(func(_ context.Context, shared any) (any, error) { return runner(shared) }))
}

func (g *Group) AddTask(task func(context.Context) error) *node {
	return g.addNode(func(ctx context.Context, _ any) error { return task(ctx) })
}

func (g *Group) AddSharedTask(task func(context.Context, any) error) *node {
	return g.addNode(task)
}

func (g *Group) AddAutoTask(task func(context.Context) (any, error)) *node {
	return g.addNode(autoWrapper(func(ctx context.Context, _ any) (any, error) { return task(ctx) }))
}

func (g *Group) AddAutoSharedTask(task func(context.Context, any) (any, error)) *node {
	return g.addNode(autoWrapper(task))
}

func (g *Group) AddNode(n Node) *node {
	return g.addNode(n.Exec).
		Key(n.Key()).
		Dep(n.Dep()...).
		WeakDep(n.WeakDep()...)
}

func (g *Group) AddAutoNode(n AutoNode) *node {
	return g.addNode(autoWrapper(n.Exec)).
		Key(n.Key()).
		Dep(n.Dep()...).
		WeakDep(n.WeakDep()...)
}

// region Batch Adding Operations

func (g *Group) AddRunners(runners ...func() error) *nodes {
	indices := make([]int, 0, len(runners))
	for _, runner := range runners {
		n := g.addNode(func(context.Context, any) error { return runner() })
		indices = append(indices, n.idx)
	}
	return &nodes{Group: g, indices: indices}
}

func (g *Group) AddSharedRunners(runners ...func(any) error) *nodes {
	indices := make([]int, 0, len(runners))
	for _, runner := range runners {
		n := g.addNode(func(_ context.Context, shared any) error { return runner(shared) })
		indices = append(indices, n.idx)
	}
	return &nodes{Group: g, indices: indices}
}

func (g *Group) AddAutoRunners(runners ...func() (any, error)) *nodes {
	indices := make([]int, 0, len(runners))
	for _, runner := range runners {
		n := g.addNode(autoWrapper(func(context.Context, any) (any, error) { return runner() }))
		indices = append(indices, n.idx)
	}
	return &nodes{Group: g, indices: indices}
}

func (g *Group) AddAutoSharedRunners(runners ...func(any) (any, error)) *nodes {
	indices := make([]int, 0, len(runners))
	for _, runner := range runners {
		n := g.addNode(autoWrapper(func(_ context.Context, shared any) (any, error) { return runner(shared) }))
		indices = append(indices, n.idx)
	}
	return &nodes{Group: g, indices: indices}
}

func (g *Group) AddTasks(tasks ...func(context.Context) error) *nodes {
	indices := make([]int, 0, len(tasks))
	for _, task := range tasks {
		n := g.addNode(func(ctx context.Context, _ any) error { return task(ctx) })
		indices = append(indices, n.idx)
	}
	return &nodes{Group: g, indices: indices}
}

func (g *Group) AddSharedTasks(tasks ...func(context.Context, any) error) *nodes {
	indices := make([]int, 0, len(tasks))
	for _, task := range tasks {
		n := g.addNode(task)
		indices = append(indices, n.idx)
	}
	return &nodes{Group: g, indices: indices}
}

func (g *Group) AddAutoTasks(tasks ...func(context.Context) (any, error)) *nodes {
	indices := make([]int, 0, len(tasks))
	for _, task := range tasks {
		n := g.addNode(autoWrapper(func(ctx context.Context, _ any) (any, error) { return task(ctx) }))
		indices = append(indices, n.idx)
	}
	return &nodes{Group: g, indices: indices}
}

func (g *Group) AddAutoSharedTasks(tasks ...func(context.Context, any) (any, error)) *nodes {
	indices := make([]int, 0, len(tasks))
	for _, task := range tasks {
		n := g.addNode(autoWrapper(task))
		indices = append(indices, n.idx)
	}
	return &nodes{Group: g, indices: indices}
}

func (g *Group) AddNodes(ns ...Node) *nodes {
	indices := make([]int, 0, len(ns))
	for _, n := range ns {
		node := g.addNode(n.Exec).
			Key(n.Key()).
			Dep(n.Dep()...).
			WeakDep(n.WeakDep()...)
		indices = append(indices, node.idx)
	}
	return &nodes{Group: g, indices: indices}
}

func (g *Group) AddAutoNodes(ns ...AutoNode) *nodes {
	indices := make([]int, 0, len(ns))
	for _, n := range ns {
		node := g.addNode(autoWrapper(n.Exec)).
			Key(n.Key()).
			Dep(n.Dep()...).
			WeakDep(n.WeakDep()...)
		indices = append(indices, node.idx)
	}
	return &nodes{Group: g, indices: indices}
}

// region Serial Adding Operations

func (g *Group) AddSerialRunners(runners ...func() error) *node {
	return g.addNode(func(context.Context, any) error {
		for _, runner := range runners {
			if err := runner(); err != nil {
				return err
			}
		}
		return nil
	})
}

func (g *Group) AddSerialSharedRunners(runners ...func(any) error) *node {
	return g.addNode(func(_ context.Context, shared any) error {
		for _, runner := range runners {
			if err := runner(shared); err != nil {
				return err
			}
		}
		return nil
	})
}

func (g *Group) AddSerialAutoRunners(runners ...func() (any, error)) *node {
	return g.addNode(func(ctx context.Context, _ any) error {
		for _, runner := range runners {
			if err := autoWrapper(func(context.Context, any) (any, error) { return runner() })(ctx, nil); err != nil {
				return err
			}
		}
		return nil
	})
}

func (g *Group) AddSerialAutoSharedRunners(runners ...func(any) (any, error)) *node {
	return g.addNode(func(ctx context.Context, shared any) error {
		for _, runner := range runners {
			if err := autoWrapper(func(_ context.Context, shared any) (any, error) { return runner(shared) })(ctx, shared); err != nil {
				return err
			}
		}
		return nil
	})
}

func (g *Group) AddSerialTasks(tasks ...func(context.Context) error) *node {
	return g.addNode(func(ctx context.Context, _ any) error {
		for _, task := range tasks {
			if err := task(ctx); err != nil {
				return err
			}
		}
		return nil
	})
}

func (g *Group) AddSerialSharedTasks(tasks ...func(context.Context, any) error) *node {
	return g.addNode(func(ctx context.Context, shared any) error {
		for _, task := range tasks {
			if err := task(ctx, shared); err != nil {
				return err
			}
		}
		return nil
	})
}

func (g *Group) AddSerialAutoTasks(tasks ...func(context.Context) (any, error)) *node {
	return g.addNode(func(ctx context.Context, _ any) error {
		for _, task := range tasks {
			if err := autoWrapper(func(context.Context, any) (any, error) { return task(ctx) })(ctx, nil); err != nil {
				return err
			}
		}
		return nil
	})
}

func (g *Group) AddSerialAutoSharedTasks(tasks ...func(context.Context, any) (any, error)) *node {
	return g.addNode(func(ctx context.Context, shared any) error {
		for _, task := range tasks {
			if err := autoWrapper(func(context.Context, any) (any, error) { return task(ctx, shared) })(ctx, shared); err != nil {
				return err
			}
		}
		return nil
	})
}

func (g *Group) AddSerialNodes(ns ...Node) *node {
	return g.addNode(func(ctx context.Context, shared any) error {
		for _, n := range ns {
			if err := n.Exec(ctx, shared); err != nil {
				return err
			}
		}
		return nil
	})
}

func (g *Group) AddSerialAutoNodes(ns ...AutoNode) *node {
	return g.addNode(func(ctx context.Context, shared any) error {
		for _, n := range ns {
			if err := autoWrapper(n.Exec)(ctx, shared); err != nil {
				return err
			}
		}
		return nil
	})
}

// region Common Operations

// Get node by key
func (g *Group) Node(key any) *node {
	if idx, ok := g.idxMap[key]; ok {
		return g.nodes[idx]
	}
	return nil
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
