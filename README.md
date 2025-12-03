# [Group] Concurrency Kit

⚡ A lightweight, dependency-aware (*yet another DAG*) concurrency toolkit built on top of std errgroup, providing fine-grained control over concurrent task execution with minimal overhead.

*Much less overhead by optional features selected*

## APIs
`Go(ctx, opts, fs...)`  `TryGo(ctx, opts, fs...) bool`

---

`Opts(With...) *Options`

---

`NewGroup() *Group`

**`(Group).Go(ctx) error`**  `(Group).Add... *node`  `(Group).Node(key) *node`  `(Group).Verify(panicking) *node`

---
`WithStore(ctx, store) context.Context`
`Store(ctx, value)`  `Fetch(ctx, key)`  `Put(ctx, key, value)`


## Options
Get options by `Opts(With...)`

or by `Options{...}`

Available options:
- `WithPrefix(string)` - Set group name for logging
- `WithLimit(int)` - Set concurrency limit
- `WithPreFunc(PreFunc)` - Set group pre-execution interceptor
- `WithAfterFunc(AfterFunc)` - Set group post-execution interceptor
- `WithTimeout(time.Duration)` - Set group timeout
- `WithErrorCollector(chan error)` - Collect errors in channel
- `WithLogger(*slog.Logger)` - Use custom logger
- `WithLog` - Enable logging

---

## Group Mode (DAG)

### Features
- **🔗 Dependency Management**: Define task dependencies with automatic execution ordering
- **🧩 Weak Dependencies**: Continue execution even when upstream tasks fail
- **📦 Built-in Store**: Share data between dependent tasks using context-based storage
- **💥 Fast-Fail Control**: Configure tasks to halt group execution on error
- **🔄 Retry Mechanism**: Configure automatic retry for individual nodes
- **🎣 Interceptors**: Pre and post-execution hooks at both group and node level
- **🔙 Rollback Mechanism**: Define compensation logic to revert changes when tasks fail
- **⏱️ Timeout Control**: Set timeouts at the group level
- **📊 Monitoring & Logging**: Optional execution monitoring and logging

#### 🍃 Error Propagation
Within a group, errors propagate according to dependency order, eventually returning only **leaf errors** that have already aggregated parent errors.
If multiple leaf errors exist, they are aggregated using `errors.Join` (when a fast-fail error occurs, only the aggregated error from the fast-fail node is returned).

### Usage

#### [Basic Workflow]
Create a group using `NewGroup` with optional configurations, then add tasks using `AddRunner`, `AddTask`, or `AddSharedTask`. Each task can be assigned a unique key and specify its dependencies. Finally, execute the group with `Go` method.

#### [Task Types]
***🚀 Simple Runner*** - Basic function that returns an error. No access to context or shared state.

***🚁 Context-Aware Task*** - Receives a context parameter, allowing the task to respond to cancellation signals and timeouts.
Context-aware tasks will be able to communicate data through `Store` and `Fetch`. Additionally, you can directly insert key-value pairs into the context by using `Put`.

***🚢 Shared-State Task*** - Receives both context and a shared state object, enabling tasks to access and modify common data structures.
shared-state task will be able to access predefined shared data via the shared arguments passed in (**❗❗ beware of potential data race**).

#### [Node Configuration]
- **`Key(any)`** - Assign unique identifier
- **`Dep(...any)`** - Add strong dependencies (blocks on upstream errors)
- **`WeakDep(...any)`** - Add weak dependencies (continues on upstream errors)
- **`FastFail()`** - Halt entire group on node error
- **`WithRetry(int)`** - Set retry attempts on failure
- **`WithPreFunc(NodePreFunc)`** - Set node pre-execution interceptor
- **`WithAfterFunc(NodeAfterFunc)`** - Set node post-execution interceptor
- **`WithRollback(RollbackFunc)`** - Set compensation function executed on failure
- **`WithTimeout(time.Duration)`** - Set node-specific timeout

#### [More...]
Refer to the example package in this repo

### Verify
Verify checks for cycles in the dependency graph by using `group.Verify()` or `Node.Verify()`

---

## Benchmark
```
goos: darwin
goarch: arm64
pkg: github.com/oatcatx/group/benchmark
cpu: Apple M3 Pro
BenchmarkGo

BenchmarkGo/TinyWorkload
BenchmarkGo/TinyWorkload/StdGoroutine
BenchmarkGo/TinyWorkload/StdGoroutine-12         	   78626	     15192 ns/op	     259 B/op	      11 allocs/op
BenchmarkGo/TinyWorkload/StdErrGroup
BenchmarkGo/TinyWorkload/StdErrGroup-12          	   50954	     26485 ns/op	     400 B/op	      13 allocs/op
BenchmarkGo/TinyWorkload/Go
BenchmarkGo/TinyWorkload/Go-12                   	   40836	     29868 ns/op	    1105 B/op	      25 allocs/op
BenchmarkGo/TinyWorkload/GoWithOpts
BenchmarkGo/TinyWorkload/GoWithOpts-12           	   20181	     62942 ns/op	    5218 B/op	     102 allocs/op

BenchmarkGo/SmallWorkload
BenchmarkGo/SmallWorkload/StdGoroutine
BenchmarkGo/SmallWorkload/StdGoroutine-12        	     832	   1253898 ns/op	   12129 B/op	     201 allocs/op
BenchmarkGo/SmallWorkload/StdErrGroup
BenchmarkGo/SmallWorkload/StdErrGroup-12         	     956	   1277130 ns/op	   12197 B/op	     203 allocs/op
BenchmarkGo/SmallWorkload/Go
BenchmarkGo/SmallWorkload/Go-12                  	     922	   1318295 ns/op	   17186 B/op	     305 allocs/op
BenchmarkGo/SmallWorkload/GoWithOpts
BenchmarkGo/SmallWorkload/GoWithOpts-12          	     849	   1446455 ns/op	   49307 B/op	     946 allocs/op

BenchmarkGo/MediumWorkload
BenchmarkGo/MediumWorkload/StdGoroutine
BenchmarkGo/MediumWorkload/StdGoroutine-12       	     184	   6491799 ns/op	  122766 B/op	    2005 allocs/op
BenchmarkGo/MediumWorkload/StdErrGroup
BenchmarkGo/MediumWorkload/StdErrGroup-12        	     182	   6548971 ns/op	  120220 B/op	    2003 allocs/op
BenchmarkGo/MediumWorkload/Go
BenchmarkGo/MediumWorkload/Go-12                 	     163	   7078103 ns/op	  168493 B/op	    3005 allocs/op
BenchmarkGo/MediumWorkload/GoWithOpts
BenchmarkGo/MediumWorkload/GoWithOpts-12         	     165	   7278053 ns/op	  480847 B/op	    9282 allocs/op

BenchmarkGo/LargeWorkload
BenchmarkGo/LargeWorkload/StdGoroutine
BenchmarkGo/LargeWorkload/StdGoroutine-12               21	  52983192 ns/op	 1290996 B/op	   20188 allocs/op
BenchmarkGo/LargeWorkload/StdErrGroup
BenchmarkGo/LargeWorkload/StdErrGroup-12         	      13	  82499465 ns/op	 1239411 B/op	   20183 allocs/op
BenchmarkGo/LargeWorkload/Go
BenchmarkGo/LargeWorkload/Go-12                  	      15	  71173006 ns/op	 1680838 B/op	   30009 allocs/op
BenchmarkGo/LargeWorkload/GoWithOpts
BenchmarkGo/LargeWorkload/GoWithOpts-12          	      14	  73112423 ns/op	 4790445 B/op	   92600 allocs/op
```

---

```
goos: darwin
goarch: arm64
pkg: github.com/oatcatx/group/benchmark
cpu: Apple M3 Pro
BenchmarkGroup

BenchmarkGroup/StdErrGroup
BenchmarkGroup/StdErrGroup-12         	   79412	     18070 ns/op	     641 B/op	      17 allocs/op
BenchmarkGroup/Group
BenchmarkGroup/Group-12               	   50367	     24134 ns/op	    2680 B/op	      45 allocs/op
```
