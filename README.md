# [Group] Concurrency Kit

⚡ A lightweight, dependency-aware (*yet another DAG*) concurrency toolkit built on top of std errgroup, providing fine-grained control over concurrent task execution with minimal overhead.

*Much less overhead by optional features selected*

## APIs
`group.Go(), group.TryGo()`

---

`group.Opts(group.With...)`

---

`group.NewGroup`

`(Group).Add...`  **`Group.Go`**  `(Group).Node`  `(Group).Verify`

`Node.Key`  `Node.Dep`  `Node.WeakDep`  `Node.FF`  `Node.Verify`

## Options
Get options by `group.Opts(group.With...)`

or by `group.Options{...}`

Available options:
- `WithPrefix(string)` - Set group name for logging
- `WithLimit(int)` - Set concurrency limit
- `WithTimeout(time.Duration)` - Set group timeout
- `WithLog` - Enable logging
- `WithLogger(*slog.Logger)` - Use custom logger
- `WithErrorCollector(chan error)` - Collect errors in channel

---

## Group Mode (DAG)

### Features

- **🔗 Dependency Management**: Define task dependencies with automatic execution ordering
- **🧩 Weak Dependencies**: Continue execution even when upstream tasks fail
- **💥 Fast-Fail Control**: Configure tasks to halt group execution on error
- **📦 Built-in Store**: Share data between dependent tasks using context-based storage
- **⏱️ Timeout Control**: Set timeouts at the group level
- **📊 Monitoring & Logging**: Optional execution monitoring and logging

#### Error Propagation
Within a group, errors propagate according to dependency order, eventually returning only **leaf errors** that have already aggregated parent errors.
If multiple leaf errors exist, they are aggregated using `errors.Join` (when a fast-fail error occurs, only the aggregated error from the fast-fail node is returned).

### Usage

#### [Basic Workflow]

Create a group using `NewGroup` with optional configurations, then add tasks using `AddRunner`, `AddTask`, or `AddSharedTask`. Each task can be assigned a unique key and specify its dependencies. Finally, execute the group with `Go` method.

#### [Task Types]

***🚀 Simple Runner*** - Basic function that returns an error. No access to context or shared state.

***🚁 Context-Aware Task*** - Receives a context parameter, allowing the task to respond to cancellation signals and timeouts.
Context-aware tasks will be able to communicate data through `Store` and `Fetch`.

***🚢 Shared-Data Task*** - Receives both context and a shared state object, enabling tasks to access and modify common data structures.
shared-state task will be able to access predefined shared data via the shared arguments passed in (**❗❗ beware of potential data race**).

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
BenchmarkGo/TinyWorkload/StdGoroutine-12         	   85225	     14676 ns/op	     258 B/op	      11 allocs/op
BenchmarkGo/TinyWorkload/StdErrGroup
BenchmarkGo/TinyWorkload/StdErrGroup-12          	   56398	     20374 ns/op	     400 B/op	      13 allocs/op
BenchmarkGo/TinyWorkload/Go
BenchmarkGo/TinyWorkload/Go-12                   	   43058	     27641 ns/op	    1105 B/op	      25 allocs/op
BenchmarkGo/TinyWorkload/GoWithOpts
BenchmarkGo/TinyWorkload/GoWithOpts-12           	   20499	     58025 ns/op	    5015 B/op	      91 allocs/op

BenchmarkGo/SmallWorkload
BenchmarkGo/SmallWorkload/StdGoroutine
BenchmarkGo/SmallWorkload/StdGoroutine-12        	     991	   1271535 ns/op	   12149 B/op	     201 allocs/op
BenchmarkGo/SmallWorkload/StdErrGroup
BenchmarkGo/SmallWorkload/StdErrGroup-12         	     920	   1238819 ns/op	   12175 B/op	     203 allocs/op
BenchmarkGo/SmallWorkload/Go
BenchmarkGo/SmallWorkload/Go-12                  	     880	   1298515 ns/op	   17186 B/op	     305 allocs/op
BenchmarkGo/SmallWorkload/GoWithOpts
BenchmarkGo/SmallWorkload/GoWithOpts-12          	     876	   1405319 ns/op	   47737 B/op	     845 allocs/op

BenchmarkGo/MediumWorkload
BenchmarkGo/MediumWorkload/StdGoroutine
BenchmarkGo/MediumWorkload/StdGoroutine-12       	     181	   6559549 ns/op	  122747 B/op	    2006 allocs/op
BenchmarkGo/MediumWorkload/StdErrGroup
BenchmarkGo/MediumWorkload/StdErrGroup-12        	     183	   6899276 ns/op	  120242 B/op	    2003 allocs/op
BenchmarkGo/MediumWorkload/Go
BenchmarkGo/MediumWorkload/Go-12                 	     181	   6719674 ns/op	  168533 B/op	    3005 allocs/op
BenchmarkGo/MediumWorkload/GoWithOpts
BenchmarkGo/MediumWorkload/GoWithOpts-12         	     165	   7242332 ns/op	  464363 B/op	    8276 allocs/op

BenchmarkGo/LargeWorkload
BenchmarkGo/LargeWorkload/StdGoroutine
BenchmarkGo/LargeWorkload/StdGoroutine-12        	      22	  51806746 ns/op	 1277066 B/op	   20161 allocs/op
BenchmarkGo/LargeWorkload/StdErrGroup
BenchmarkGo/LargeWorkload/StdErrGroup-12         	      21	  53904347 ns/op	 1213831 B/op	   20037 allocs/op
BenchmarkGo/LargeWorkload/Go
BenchmarkGo/LargeWorkload/Go-12                  	      18	  61684032 ns/op	 1694099 B/op	   30037 allocs/op
BenchmarkGo/LargeWorkload/GoWithOpts
BenchmarkGo/LargeWorkload/GoWithOpts-12          	      18	  66239204 ns/op	 4632736 B/op	   82634 allocs/op
```

```
goos: darwin
goarch: arm64
pkg: github.com/oatcatx/group/benchmark
cpu: Apple M3 Pro
BenchmarkGroupGo

BenchmarkGroupGo/StdErrGroup
BenchmarkGroupGo/StdErrGroup-12         	   63068	     18095 ns/op	     641 B/op	      17 allocs/op
BenchmarkGroupGo/GroupGo
BenchmarkGroupGo/GroupGo-12             	   51577	     23430 ns/op	    2432 B/op	      40 allocs/op
```