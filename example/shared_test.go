package group

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/oatcatx/group"
)

type sharedUnit struct {
	mark string
	sync.RWMutex
}

func TestGroupGoShared(t *testing.T) {
	t.Parallel()

	t.Run("single shared", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		var shared = &sharedUnit{mark: "INIT"}
		var taskA = func(ctx context.Context, shared any) error {
			if shared, _ := shared.(*sharedUnit); shared != nil {
				fmt.Print("task A reading shared unit: ")
				shared.RLock()
				fmt.Println(shared.mark)
				shared.RUnlock()

				fmt.Println("task A writing shared unit...")
				shared.Lock()
				shared.mark = "A"
				shared.Unlock()
			} else {
				fmt.Println("task A received unexpected argument type")
				fmt.Println(shared)
			}
			return nil
		}
		var taskB = func(ctx context.Context, shared any) error {
			if shared, _ := shared.(*sharedUnit); shared != nil {
				fmt.Print("task B reading shared unit: ")
				shared.RLock()
				fmt.Println(shared.mark)
				shared.RUnlock()

				fmt.Println("task B writing shared unit...")
				shared.Lock()
				shared.mark = "B"
				shared.Unlock()
			}
			return nil
		}

		err := NewGroup().
			AddSharedTask(taskA).
			AddSharedTask(taskB).
			Go(ctx, shared)

		assert.Nil(t, err)
		assert.Contains(t, []string{"A", "B"}, shared.mark) // might be A or B
	})

	t.Run("multiple shareds", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()

		var shared1, shared2 = &sharedUnit{mark: "INIT_1"}, &sharedUnit{mark: "INIT_2"}

		var taskA = func(ctx context.Context, shareds any) error {
			if shareds, _ := shareds.([]any); len(shareds) == 2 {
				if shared1, _ := shareds[0].(*sharedUnit); shared1 != nil {
					fmt.Print("task A reading shared unit 1: ")
					shared1.RLock()
					fmt.Println(shared1.mark)
					shared1.RUnlock()

					fmt.Println("task A writing shared unit 1...")
					shared1.Lock()
					shared1.mark = "A"
					shared1.Unlock()
				}
				if shared2, _ := shareds[1].(*sharedUnit); shared2 != nil {
					fmt.Print("task A reading shared unit 2: ")
					shared2.RLock()
					fmt.Println(shared2.mark)
					shared2.RUnlock()

					fmt.Println("task A writing shared unit 2...")
					shared2.Lock()
					shared2.mark = "A"
					shared2.Unlock()
				}
			}
			return nil
		}
		var taskB = func(ctx context.Context, shareds any) error {
			if shareds, _ := shareds.([]any); len(shareds) == 2 {
				if shared1, _ := shareds[0].(*sharedUnit); shared1 != nil {
					fmt.Print("task B reading shared unit 1: ")
					shared1.RLock()
					fmt.Println(shared1.mark)
					shared1.RUnlock()

					fmt.Println("task B writing shared unit 1...")
					shared1.Lock()
					shared1.mark = "B"
					shared1.Unlock()
				}
				if shared2, _ := shareds[1].(*sharedUnit); shared2 != nil {
					fmt.Print("task B reading shared unit 2: ")
					shared2.RLock()
					fmt.Println(shared2.mark)
					shared2.RUnlock()

					fmt.Println("task B writing shared unit 2...")
					shared2.Lock()
					shared2.mark = "B"
					shared2.Unlock()
				}
			}
			return nil
		}

		err := NewGroup().
			AddSharedTask(taskA).
			AddSharedTask(taskB).
			Go(ctx, shared1, shared2)

		assert.Nil(t, err)
		assert.Contains(t, []string{"A", "B"}, shared1.mark) // might be A or B
		assert.Contains(t, []string{"A", "B"}, shared2.mark) // might be A or B
	})
}
