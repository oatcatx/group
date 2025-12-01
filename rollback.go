package group

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
)

type rollbackTracker struct {
	order []*node
	cnt   uint32
}

func (r *rollbackTracker) track(n *node) {
	r.order[atomic.AddUint32(&r.cnt, 1)-1] = n
}

func (r *rollbackTracker) rollback(ctx context.Context, shared any, groupErrs []error) error {
	total := atomic.LoadUint32(&r.cnt)
	if total == 0 {
		return nil
	}
	var errs []error
	ctx = context.WithoutCancel(ctx)
	for i := int(total) - 1; i >= 0; i-- {
		n := r.order[i]
		if err := n.rollback(ctx, shared, groupErrs[n.idx]); err != nil {
			errs = append(errs, fmt.Errorf("rollback %v failed: %w", n.key, err))
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}
