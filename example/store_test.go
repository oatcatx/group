package group

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	. "github.com/oatcatx/group"
)

func TestGroupGoStore(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	type (
		A struct{}
		B struct{}
		C struct{}
		D struct{}
	)

	var taskA = func(ctx context.Context) error {
		time.Sleep(1 * time.Second)
		fmt.Println("A")
		Store(ctx, 1) // a = 1
		return nil
	}
	var taskB = func(ctx context.Context) error {
		time.Sleep(1 * time.Second)
		fmt.Println("B")
		aRes, ok := Fetch[int](ctx, A{})
		if ok {
			fmt.Println("B fetched A:", aRes)
		} else {
			panic("Fetch A Failed")
		}
		Store(ctx, aRes+1) // b = a + 1
		return nil
	}
	var taskC = func(ctx context.Context) error {
		time.Sleep(1 * time.Second)
		fmt.Println("C")
		aRes, ok := Fetch[int](ctx, A{})
		if ok {
			fmt.Println("C fetched A:", aRes)
		} else {
			panic("Fetch A Failed")
		}
		Store(ctx, aRes+1) // c = a + 1
		return nil
	}
	var taskD = func(ctx context.Context) error {
		time.Sleep(1 * time.Second)
		fmt.Println("D")
		bRes, ok := Fetch[int](ctx, B{})
		if ok {
			fmt.Println("D fetched B:", bRes)
		} else {
			panic("Fetch B Failed")
		}
		cRes, ok := Fetch[int](ctx, C{})
		if ok {
			fmt.Println("D fetched C:", cRes)
		} else {
			panic("Fetch C Failed")
		}
		Store(ctx, bRes+cRes) // d = b + c
		return nil
	}

	ctx = WithStore(ctx, &sync.Map{})
	NewGroup().
		AddTask(taskA).Key(A{}).
		AddTask(taskB).Key(B{}).Dep(A{}).
		AddTask(taskC).Key(C{}).Dep(A{}).
		AddTask(taskD).Key(D{}).Dep(B{}, C{}).
		Go(ctx)

	res, ok := Fetch[int](ctx, D{})
	assert.True(t, ok)
	assert.Equal(t, 4, res)
}
