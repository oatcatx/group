package group

import (
	"errors"
	"fmt"
	"strings"
)

type groupError struct {
	err       error
	upstreams []error
}

func (e *groupError) Error() string {
	if len(e.upstreams) == 0 {
		return e.err.Error()
	}
	if len(e.upstreams) == 1 {
		return fmt.Sprintf("%v <- %v", e.err, e.upstreams[0])
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%v <- [", e.err)
	for i, up := range e.upstreams {
		if i > 0 {
			b.WriteString(" | ")
		}
		b.WriteString(up.Error())
	}
	b.WriteString("]")
	return b.String()
}

func (e *groupError) Unwrap() []error {
	return append([]error{e.err}, e.upstreams...)
}

func wrapError(n *node, err error, groupErrs []error) error {
	if len(n.deps) == 0 {
		return err
	}
	var upstreamErrs []error
	for _, depIdx := range n.deps {
		if e := groupErrs[depIdx]; e != nil {
			upstreamErrs = append(upstreamErrs, e)
		}
	}
	if len(upstreamErrs) == 0 {
		return err
	}
	return &groupError{err: err, upstreams: upstreamErrs}
}

func leafError(nodes []*node, groupErrs []error) error {
	var leafErrs []error
	for _, n := range nodes {
		if groupErrs[n.idx] == nil {
			continue
		}
		leaf := true
		for _, toIdx := range n.to {
			if groupErrs[toIdx] != nil {
				leaf = false
				break
			}
		}
		if leaf {
			leafErrs = append(leafErrs, groupErrs[n.idx])
		}
	}
	if len(leafErrs) == 0 {
		return nil
	}
	if len(leafErrs) == 1 {
		return leafErrs[0]
	}
	return errors.Join(leafErrs...)
}
