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

// Use [Add...] methods to add different types of nodes to the group
/*
 * [AddAuto...] adds an auto node that returns a value and automatically stores it in the context store
 * CAUTION: will PANIC if auto nodes are not used with storer-context
 */
func NewGroup(opts ...option) *Group {
	g := &Group{
		nodes:   make([]*node, 0),
		idxMap:  make(map[any]int),
		Options: *Opts(opts...),
	}
	if g.prefix == "" {
		g.prefix = "anonymous" // default prefix
	}
	return g
}

// Go runs the group with added nodes
/*
 * if shared units are provided, they will be passed to the shared nodes
 * if len(shared) == 1, the node receives shared[0] (type any)
 * if len(shared) > 1, the node receives shared (type []any)
 * multiple shared units are not recommended
 */
func (g *Group) Go(ctx context.Context, shared ...any) (err error) {
	if len(g.nodes) == 0 {
		return nil
	}

	if g.log {
		defer func(start time.Time) {
			groupMonitor(ctx, "Group.Go", g.prefix, start, g.log, err)
		}(time.Now())
	}

	limit := len(g.nodes) // limit defaults to the number of nodes
	if g.limit > 0 {
		limit = g.limit
	}

	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(limit)

	// group timeout
	if g.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, g.timeout)
		defer cancel()
	}

	// group pre-execution interceptor
	if g.pre != nil {
		if err = g.pre(ctx); err != nil {
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
		if g.after != nil {
			err = g.after(ctx, err)
		}
	}()

	// outer timeout control
	if g.timeout > 0 {
		done := make(chan error, 1)
		go func() {
			done <- eg.Wait()
		}()
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) { // actual timeout
				if g.log {
					slog.InfoContext(ctx, fmt.Sprintf("[Group::Group.Go] group %s timeout", g.prefix), slog.Duration("after", g.timeout))
				}
				return fmt.Errorf("group %s timeout", g.prefix)
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

				// error handling
				ok := err == nil
				if !ok {
					if !n.sf { // record non-silent-fail error
						groupErrs[n.idx] = wrapError(n, err, groupErrs)
					}
					if n.ff {
						if n.sf {
							err = context.Canceled // sentinel error for silent-fast-fail
						} else {
							err = groupErrs[n.idx] // fast-fail will cancel the group context with current node's error chain
						}
						return
					}
					err = nil // clear non-fast-fail error
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

			if g.log || g.ErrC != nil {
				defer func(start time.Time) {
					nodeMonitor(ctx, g.prefix, n.key, start, g.log, g.ErrC, err)
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
						select {
						case <-ctx.Done():
							return ctx.Err()
						default:
						}
						if err = retryF(ctx, shared); err == nil {
							break
						}
						if g.log && i < n.retry {
							slog.InfoContext(ctx, fmt.Sprintf("[Group::node -> exec] group %s: node %s retry #%d", g.prefix, n.key, i+1))
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
						if g.log {
							slog.InfoContext(ctx, fmt.Sprintf("[Group::node -> exec] group %s: node %s timeout", g.prefix, n.key), slog.Duration("after", g.timeout))
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
