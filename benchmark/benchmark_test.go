package benchmark

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"

	. "github.com/oatcatx/group"
)

// region TASK GEN
// Task represents a unit of work to benchmark
type Task struct {
	ID       int
	Workload int           // cpu intensity
	IO       time.Duration // i/o waiting simulation
}

func genTasks(cnt int, workload int, io time.Duration) []Task {
	tasks := make([]Task, cnt)
	for i := range cnt {
		tasks[i] = Task{
			ID:       i,
			Workload: workload,
			IO:       io,
		}
	}
	return tasks
}

func process(task Task) error {
	res := 0
	for i := range task.Workload {
		res += i
	}
	if task.IO > 0 {
		time.Sleep(task.IO)
	}
	return nil
}

func setupFuncs(tasks []Task) []func() error {
	fs := make([]func() error, len(tasks))
	for i, task := range tasks {
		fs[i] = func() error { return process(task) }
	}
	return fs
}

func init() {
	// discard log output
	slog.SetDefault(slog.New(slog.DiscardHandler))
}

var cases = []struct {
	name          string
	cnt, workload int
	io            time.Duration
}{
	{"Tiny", 10, 100, 0 * time.Millisecond},
	{"Small", 100, 1000, 1 * time.Millisecond},
	{"Medium", 1000, 10000, 5 * time.Millisecond},
	{"Large", 10000, 100000, 10 * time.Millisecond},
}

// region GO
//= BENCHMARK - Go

func BenchmarkGo(b *testing.B) {
	fmt.Println()
	for _, c := range cases {
		b.Run(fmt.Sprintf("%sWorkload", c.name), func(b *testing.B) {
			runBenchmarkGo(b, genTasks(c.cnt, c.workload, c.io))
		})
		fmt.Println()
	}
}

func runBenchmarkGo(b *testing.B, tasks []Task) {
	b.Run("StdGoroutine", func(b *testing.B) {
		fs := setupFuncs(tasks)
		loopStdGoroutine(b, fs...)
	})

	b.Run("StdErrGroup", func(b *testing.B) {
		fs := setupFuncs(tasks)
		loopStdErrGroup(b, fs...)
	})

	b.Run("Go", func(b *testing.B) {
		fs := setupFuncs(tasks)
		loopGo(b, nil, fs...)
	})

	b.Run("GoWithOpts", func(b *testing.B) {
		fs := setupFuncs(tasks)
		loopGo(b, Opts(WithTimeout(1*time.Minute), WithLog), fs...)
	})
}

func loopStdGoroutine(b *testing.B, fs ...func() error) {
	for b.Loop() {
		var wg sync.WaitGroup
		wg.Add(len(fs))
		for _, f := range fs {
			go func() {
				defer wg.Done()
				_ = f()
			}()
		}
		wg.Wait()
	}
}

func loopStdErrGroup(b *testing.B, fs ...func() error) {
	for b.Loop() {
		g, _ := errgroup.WithContext(context.Background())
		for _, f := range fs {
			g.Go(f)
		}
		_ = g.Wait()
	}
}

func loopGo(b *testing.B, opts *Options, fs ...func() error) {
	for b.Loop() {
		_ = Go(context.Background(), opts, fs...)
	}
}

// region GROUP
//= BENCHMARK - Group

type benchmarkCtx struct {
	a, b, c, d int
}

func (c *benchmarkCtx) A() error {
	c.a = 1
	return nil
}

func (c *benchmarkCtx) B() error {
	c.b = c.a + 1
	return nil
}

func (c *benchmarkCtx) C() error {
	c.c = c.a + 1
	return nil
}

func (c *benchmarkCtx) D() error {
	c.d = c.b + c.c
	return nil
}

func BenchmarkGroup(b *testing.B) {
	fmt.Println()
	b.Run("StdErrGroup", func(b *testing.B) {
		loopStdErrGroupDep(b, new(benchmarkCtx))
	})

	b.Run("Group", func(b *testing.B) {
		loopGroup(b, new(benchmarkCtx))
	})
	fmt.Println()
}

func loopStdErrGroupDep(b *testing.B, c *benchmarkCtx) {
	for b.Loop() {
		g, _ := errgroup.WithContext(context.Background())
		g.Go(c.A)
		_ = g.Wait()
		g, _ = errgroup.WithContext(context.Background())
		g.Go(c.B)
		g.Go(c.C)
		_ = g.Wait()
		g, _ = errgroup.WithContext(context.Background())
		g.Go(c.D)
		_ = g.Wait()
	}
}

type (
	A struct{}
	B struct{}
	C struct{}
	D struct{}
)

func loopGroup(b *testing.B, c *benchmarkCtx) {
	for b.Loop() {
		_ = NewGroup().
			AddRunner(c.A).Key(A{}).
			AddRunner(c.B).Key(B{}).Dep(A{}).
			AddRunner(c.C).Key(C{}).Dep(A{}).
			AddRunner(c.D).Key(D{}).Dep(B{}, C{}).
			Go(context.Background())
	}
}
