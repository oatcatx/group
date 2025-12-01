package group

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/oatcatx/group"
	"github.com/stretchr/testify/assert"
)

type (
	A struct{}
	B struct{}
)

type nodeA struct{}

func (nodeA) Key() any {
	return A{}
}

func (nodeA) Dep() []any {
	return nil
}

func (nodeA) WeakDep() []any {
	return nil
}

func (nodeA) Exec(ctx context.Context, shared any) error {
	time.Sleep(1 * time.Second)
	fmt.Println("Interface Node A")
	return nil
}

var _ Node = nodeA{}

type nodeB struct{}

func (nodeB) Key() any {
	return B{}
}

func (nodeB) Dep() []any {
	return []any{A{}}
}

func (nodeB) WeakDep() []any {
	return nil
}

func (nodeB) Exec(ctx context.Context, shared any) error {
	time.Sleep(1 * time.Second)
	fmt.Println("Interface Node B")
	return nil
}

var _ Node = nodeB{}

func TestGroupGoInterfaceNode(t *testing.T) {
	t.Parallel()
	ctx, s := context.Background(), time.Now()

	err := NewGroup().
		AddNode(nodeA{}).
		AddNode(nodeB{}).
		Go(ctx)

	assert.Nil(t, err)
	assert.Equal(t, float64(2), time.Since(s).Truncate(time.Second).Seconds())
}
