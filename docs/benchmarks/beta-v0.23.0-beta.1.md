# Benchmarks: nanostack-framework v0.23.0-beta.1

Anchor on `feat/framework-beta-nano` at `43375b51` (nanostack-framework
v0.23.0-beta.1, Nanostack DB, slog logging, nano generated wiring, chi kept)
against [baseline-v0.20.0](baseline-v0.20.0.md). Recorded 2026-11-08 with the
baseline command on the same machine:

```sh
cd apps/anchor
go test -tags contract -run '^$' -bench . -benchmem -count=10 -timeout 60m ./cmd/it/contract/
go run golang.org/x/perf/cmd/benchstat@latest baseline-v0.20.0.txt beta-v0.23.0-beta.1.txt
```

Machine: Apple M2 Max (darwin/arm64), go1.27.3, Docker 29.7.2 (Docker
Desktop), TimescaleDB 2.23.0-pg18 and Redis 7. Same as the baseline.

Files:

- [`beta-v0.23.0-beta.1.txt`](beta-v0.23.0-beta.1.txt): the full run
  (`-count=10`, all benchmarks). benchstat below compares this file.
- [`beta-v0.23.0-beta.1-rerun.txt`](beta-v0.23.0-beta.1-rerun.txt): the four
  rows that run 1 flagged, run again 20 minutes later (`-count=10`).
- [`beta-v0.23.0-beta.1-paired.txt`](beta-v0.23.0-beta.1-paired.txt):
  `ReadOrganization` and `NotFound` interleaved, 8 rounds of `-count=2`, so
  both see the same load.

## Environment: the machine was not idle

The baseline asks for an idle machine. This run did not get one, and waiting
did not help: other agents in the same workflow ran the echopoint `cmd/it/ct`
suite and `go test -race` on the echopoint packages in a loop, on the same
Docker Desktop VM that holds this suite's Postgres and Redis. During that work
the Docker VM process used up to 520% CPU.

| Run | Start | Load average (1, 5, 15 min) at start | At end |
|---|---|---|---|
| Run 1 (full suite) | 11:23 | 10.97, 10.46, 10.17 | 8.78, 10.22, 10.17 |
| Run 2 (rerun of 4 rows) | 11:48 | 6.65, 7.98, 8.21 | 11.62, 8.83, 8.49 |
| Paired (8 rounds) | 11:52 | 11.20 to 13.35 (1 min) | 10.54 |

The baseline doc does not record its load. The time rows of this report are
therefore not a clean comparison: run 1 and run 2 measure the same code and
differ by up to 62% (`Health`). The allocation, memory and startup rows are
stable under load and are a clean comparison.

## Summary

Time is the median of 10 (± is benchstat's 95% interval). Δ is benchstat's
delta against the baseline; `~` means p ≥ 0.05.

| Benchmark | Baseline v0.20.0 | Beta run 1 | Δ run 1 | Beta run 2 | Δ run 2 | B/op (base → beta) | allocs/op (base → beta) | Reading |
|---|---:|---:|---:|---:|---:|---|---|---|
| `Health` | 256.9 µs ± 7% | 754.9 µs ± 48% | +193.9% (p=0.000) | 286.6 µs ± 4% | +11.6% (p=0.001) | 15.93 → 15.70 KiB (−1.4%) | 205 → 210 (+2.4%) | Same code moved 755 → 287 µs between runs: the time is load. +5 allocs come from `core/log.HTTPMiddleware` opening the request scope (about 1 µs). |
| `ReadOrganization` | 2.015 ms ± 39% | 2.474 ms ± 18% | +22.8% (p=0.023) | 1.670 ms ± 10% | −17.1% (p=0.000) | 193.1 → 192.5 KiB (~) | 1,944 → 1,920 (−1.2%) | Flips sign between runs; no regression. 24 fewer allocs. |
| `SearchProductUsers` | 2.593 ms ± 7% | 3.073 ms ± 12% | +18.5% (p=0.002) | 2.710 ms ± 58% | ~ (p=0.912) | 245.0 → 248.2 KiB (+1.3%) | 2,835 → 2,859 (+0.9%) | Not confirmed by run 2; +24 allocs on a 20-item page is noise-level. |
| `CreateProductUser` | 6.433 ms ± 31% | 8.058 ms ± 27% | ~ (p=0.075) | - | - | 2.889 → 2.425 MiB (~) | 39.8 k → 32.7 k (~) | Same drift as the baseline (the events worker); no significant change. |
| `ValidationError` | 1.342 ms ± 30% | 1.484 ms ± 8% | ~ (p=0.075) | - | - | 119.1 → 122.0 KiB (+2.4%) | 1,266 → 1,286 (+1.6%) | No significant time change; +20 allocs (not profiled; the `HTTPMiddleware` +5 seen on `Health` is part of it). |
| `NotFound` | 1.758 ms ± 9% | 2.243 ms ± 42% | +27.6% (p=0.000) | 2.283 ms ± 63% | +29.9% (p=0.000) | 189.0 → 189.5 KiB (~) | 1,895 → 1,871 (−1.3%) | Above the threshold in both runs. Interleaved with `ReadOrganization` under the same load it is 2.00 ms against 1.97 ms. Absolute time is load; the shift relative to `ReadOrganization` stays open (see below). |
| `StartupToReady` | 184.6 ms ± 2% | 177.2 ms ± 5% | −4.0% (p=0.015) | - | - | (client only) | (client only) | Warm start 4% faster, even under load. Cold sample: 1.219 s (baseline 1.208 s). |
| `HeapAfterWarmup` | - | - | - | - | - | - | - | Time/op is meaningless; see the memory table. |

| Memory | Baseline v0.20.0 | Beta | Δ | Reading |
|---|---:|---:|---:|---|
| Process RSS when ready (`StartupToReady`, `ps rss`) | 75.03 MiB ± 2% | 64.47 MiB ± 1% | −14.1% (p=0.000) | 10.6 MiB less resident memory at start. |
| Process RSS after 400 warm-up requests | 78.68 MiB ± 1% | 68.88 MiB ± 0% | −12.4% (p=0.000) | The saving holds after warm-up. |
| Test-process heap in use after 1,000 mixed requests and 2 GCs | 28.01 MiB ± 0% | 22.29 MiB ± 0% | −20.4% (p=0.000) | 5.7 MiB less live heap. |
| Test-process heap alloc, same point | 16.69 MiB ± 0% | 13.15 MiB ± 0% | −21.2% (p=0.000) | Same reading. |
| Goroutines, same point | 26 ± 12% | 26 ± 0% | ~ | No goroutine leak or new background worker. |

The memory and startup rows are the clean result of the migration. The fx
graph is gone and the generated wiring builds the same components, so less
runtime state is retained. This report did not profile the heap to split the
saving between the wiring, the logger and Nanostack DB.

## Regressions and causes

The rule: a row that regresses by more than 10% with p < 0.05 is profiled.
Run 1 flags `Health`, `ReadOrganization`, `SearchProductUsers` and
`NotFound`. Run 2 confirms only `Health` (+11.6%, +30 µs) and `NotFound`
(+29.9%). None of them is a measured code regression; one residual
(`NotFound` relative to `ReadOrganization`) stays open. The evidence:

1. **Run-to-run spread is larger than the deltas.** The same binary moves
   `Health` by −62%, `ReadOrganization` by −32% and `SearchProductUsers` by
   −12% between run 1 and run 2 (`benchstat beta-v0.23.0-beta.1.txt
   beta-v0.23.0-beta.1-rerun.txt`). Inside run 1, `Health` falls from 1.24 ms
   to 364 µs over its ten samples while the load drops from 11 to 8.8.
2. **Allocations are flat.** allocs/op is the stable signal (± 0% on both
   sides) and it moves by −1.3% to +2.4%, with mixed sign. A 30% time
   regression from code would show in it.
3. **The CPU profiles show waiting, not work** (`-cpuprofile`, `-count=3`
   under the same load). `Health`: 2.30 s of samples in 5.01 s (46% CPU
   busy); `NotFound`: 1.32 s in 7.24 s (18%). The top frames are
   `syscall.rawsyscalln`, `runtime.kevent` and `pthread_cond_wait/signal`:
   socket I/O and scheduler wake-ups. Flat CPU in
   `github.com/nanostack-dev/nanostack-framework/...` frames is 0.43%
   (`Health`) and 0.76% (`NotFound`); flat CPU in `anchor/...` frames is 0%.
   The Postgres and Redis round trips go through the contended Docker VM.
4. **`NotFound` does the same database work as `ReadOrganization`.** An exact
   allocation profile (`-memprofilerate=1`, 300 iterations each) shows one
   product lookup and one organization lookup per request in both
   (`transactor.QueryOptional` called exactly 300 times for the
   organization). Keep-alive is intact: 1,000 extra `NotFound` requests add
   no `TIME_WAIT` sockets. Interleaved under the same load, `NotFound`
   (median 2.00 ms, n=16) costs the same as `ReadOrganization` (1.97 ms,
   n=16), and `ReadOrganization` is 2% under its baseline. In the baseline,
   `NotFound` was 13% cheaper than `ReadOrganization`. That relative shift
   (about 250 µs) is the only part of the flagged rows this report cannot
   explain from the data. It is not CPU and not an extra round trip.
5. **`Health` +5 allocs: framework, intended, too small to see.** The
   migration replaced `requestlog.Contextualize` with
   `core/log.HTTPMiddleware` (with `SkipSummary`). Its `openRequest` builds
   the request scope through `log.WithScope`, `contextHandler.WithAttrs` and
   `redactHandler.WithAttrs`: about 6 allocations per request in the alloc
   profile, about 1 µs. It does not explain +30 µs.

Causes, per row:

- `Health`, `ReadOrganization`, `SearchProductUsers`: environment (CPU
  contention on the shared Docker VM from concurrent test suites). The
  run-to-run spread is larger than the delta, allocations are flat and the
  CPU profile is dominated by waiting. Not the framework, not the service.
- `NotFound`: the absolute time is environment for the same reasons. Its cost
  relative to `ReadOrganization` moved by about +13% (baseline ratio 0.87,
  now about 1.0), and this data does not locate that shift: same query
  count, keep-alive intact, fewer allocations. It stays **open**.

Follow-up that closes both: rerun `Health`, `ReadOrganization`,
`SearchProductUsers` and `NotFound` on an idle machine and record the load
average next to the numbers. If the `NotFound` ratio shift remains, profile
the 404 error path (the error handler, the legacy envelope and its log
line) against the baseline.

## Framework issues

None found that blocks or regresses Anchor. Two framework rows that the
framework's own report keeps above its threshold
([docs/benchmarks/v0.23.0-beta.1.md](https://github.com/nanostack-dev/nanostack-framework/blob/v0.23.0-beta.1/docs/benchmarks/v0.23.0-beta.1.md))
are on Anchor's hot path: the request summary (+34%, 2.35 µs against 1.75 µs
for zerolog) and one log line (2.10 µs against 0.71 µs). Anchor writes about
two lines per request, so they cost a few µs against requests of 0.3 to 8 ms.
They are below the resolution of this suite.

## benchstat (baseline against run 1)

```
goos: darwin
goarch: arm64
pkg: anchor/cmd/it/contract
cpu: Apple M2 Max
                      │ baseline-v0.20.0.txt │        beta-v0.23.0-beta.1.txt        │
                      │        sec/op        │    sec/op     vs base                 │
Health-12                       256.9µ ±  7%   754.9µ ± 48%  +193.86% (p=0.000 n=10)
ReadOrganization-12             2.015m ± 39%   2.474m ± 18%   +22.81% (p=0.023 n=10)
SearchProductUsers-12           2.593m ±  7%   3.073m ± 12%   +18.52% (p=0.002 n=10)
CreateProductUser-12            6.433m ± 31%   8.058m ± 27%         ~ (p=0.075 n=10)
ValidationError-12              1.342m ± 30%   1.484m ±  8%         ~ (p=0.075 n=10)
NotFound-12                     1.758m ±  9%   2.243m ± 42%   +27.63% (p=0.000 n=10)
HeapAfterWarmup-12              2.038n ±  1%   2.038n ±  0%         ~ (p=0.955 n=10)
StartupToReady-12               184.6m ±  2%   177.2m ±  5%    -4.00% (p=0.015 n=10)
geomean                         544.0µ         696.8µ         +28.11%

                      │ baseline-v0.20.0.txt │        beta-v0.23.0-beta.1.txt         │
                      │         B/op         │     B/op       vs base                 │
Health-12                    15.93Ki ±  1%     15.70Ki ±  1%  -1.44% (p=0.001 n=10)
ReadOrganization-12          193.1Ki ±  0%     192.5Ki ±  0%       ~ (p=0.063 n=10)
SearchProductUsers-12        245.0Ki ±  1%     248.2Ki ±  1%  +1.29% (p=0.000 n=10)
CreateProductUser-12         2.889Mi ± 60%     2.425Mi ± 57%       ~ (p=0.393 n=10)
ValidationError-12           119.1Ki ±  1%     122.0Ki ±  0%  +2.43% (p=0.000 n=10)
NotFound-12                  189.0Ki ±  0%     189.5Ki ±  0%       ~ (p=0.063 n=10)
HeapAfterWarmup-12             0.000 ±  0%       0.000 ±  0%       ~ (p=1.000 n=10) ¹
StartupToReady-12            239.8Ki ±  3%     237.3Ki ±  3%       ~ (p=0.684 n=10)
geomean                                    ²                  -2.03%                ²
¹ all samples are equal
² summaries must be >0 to compute geomean

                      │ baseline-v0.20.0.txt │        beta-v0.23.0-beta.1.txt        │
                      │      allocs/op       │  allocs/op    vs base                 │
Health-12                      205.0 ±  0%      210.0 ±  0%  +2.44% (p=0.000 n=10)
ReadOrganization-12           1.944k ±  0%     1.920k ±  0%  -1.23% (p=0.000 n=10)
SearchProductUsers-12         2.835k ±  0%     2.859k ±  0%  +0.85% (p=0.000 n=10)
CreateProductUser-12          39.81k ± 60%     32.74k ± 57%       ~ (p=0.393 n=10)
ValidationError-12            1.266k ±  0%     1.286k ±  0%  +1.58% (p=0.000 n=10)
NotFound-12                   1.895k ±  0%     1.871k ±  0%  -1.29% (p=0.000 n=10)
HeapAfterWarmup-12             0.000 ±  0%      0.000 ±  0%       ~ (p=1.000 n=10) ¹
StartupToReady-12             2.593k ±  4%     2.562k ±  4%       ~ (p=0.239 n=10)
geomean                                    ²                 -2.28%                ²
¹ all samples are equal
² summaries must be >0 to compute geomean

                   │ baseline-v0.20.0.txt │   beta-v0.23.0-beta.1.txt    │
                   │      goroutines      │ goroutines  vs base          │
HeapAfterWarmup-12            26.00 ± 12%   26.00 ± 0%  ~ (p=0.474 n=10)

                   │ baseline-v0.20.0.txt │        beta-v0.23.0-beta.1.txt         │
                   │    heap-alloc-MiB    │ heap-alloc-MiB  vs base                │
HeapAfterWarmup-12             16.69 ± 0%       13.15 ± 0%  -21.21% (p=0.000 n=10)

                   │ baseline-v0.20.0.txt │        beta-v0.23.0-beta.1.txt         │
                   │    heap-inuse-MiB    │ heap-inuse-MiB  vs base                │
HeapAfterWarmup-12             28.01 ± 0%       22.29 ± 0%  -20.42% (p=0.000 n=10)

                  │ baseline-v0.20.0.txt │        beta-v0.23.0-beta.1.txt        │
                  │    rss-ready-MiB     │ rss-ready-MiB  vs base                │
StartupToReady-12             75.03 ± 2%      64.47 ± 1%  -14.06% (p=0.000 n=10)

                  │ baseline-v0.20.0.txt │       beta-v0.23.0-beta.1.txt        │
                  │     rss-warm-MiB     │ rss-warm-MiB  vs base                │
StartupToReady-12             78.68 ± 1%     68.88 ± 0%  -12.44% (p=0.000 n=10)
```

## benchstat (time only, baseline against run 1 and run 2)

```
goos: darwin
goarch: arm64
pkg: anchor/cmd/it/contract
cpu: Apple M2 Max
                      │ baseline-v0.20.0.txt │        beta-v0.23.0-beta.1.txt        │     beta-v0.23.0-beta.1-rerun.txt      │
                      │        sec/op        │    sec/op     vs base                 │    sec/op     vs base                  │
Health-12                       256.9µ ±  7%   754.9µ ± 48%  +193.86% (p=0.000 n=10)   286.6µ ±  4%  +11.56% (p=0.001 n=10)
ReadOrganization-12             2.015m ± 39%   2.474m ± 18%   +22.81% (p=0.023 n=10)   1.670m ± 10%  -17.10% (p=0.000 n=10)
SearchProductUsers-12           2.593m ±  7%   3.073m ± 12%   +18.52% (p=0.002 n=10)   2.710m ± 58%        ~ (p=0.912 n=10)
CreateProductUser-12            6.433m ± 31%   8.058m ± 27%         ~ (p=0.075 n=10)
ValidationError-12              1.342m ± 30%   1.484m ±  8%         ~ (p=0.075 n=10)
NotFound-12                     1.758m ±  9%   2.243m ± 42%   +27.63% (p=0.000 n=10)   2.283m ± 63%  +29.89% (p=0.000 n=10)
HeapAfterWarmup-12              2.038n ±  1%   2.038n ±  0%         ~ (p=0.955 n=10)
StartupToReady-12               184.6m ±  2%   177.2m ±  5%    -4.00% (p=0.015 n=10)
geomean                         544.0µ         696.8µ         +28.11%                  1.312m         +5.86%                ¹
¹ benchmark set differs from baseline; geomeans may not be comparable
```

## benchstat (time only, run 1 against run 2: same code)

```
goos: darwin
goarch: arm64
pkg: anchor/cmd/it/contract
cpu: Apple M2 Max
                      │ beta-v0.23.0-beta.1.txt │     beta-v0.23.0-beta.1-rerun.txt      │
                      │         sec/op          │    sec/op     vs base                  │
Health-12                          754.9µ ± 48%   286.6µ ±  4%  -62.04% (p=0.000 n=10)
ReadOrganization-12                2.474m ± 18%   1.670m ± 10%  -32.50% (p=0.000 n=10)
SearchProductUsers-12              3.073m ± 12%   2.710m ± 58%  -11.80% (p=0.029 n=10)
CreateProductUser-12               8.058m ± 27%
ValidationError-12                 1.484m ±  8%
NotFound-12                        2.243m ± 42%   2.283m ± 63%        ~ (p=0.796 n=10)
HeapAfterWarmup-12                 2.038n ±  0%
StartupToReady-12                  177.2m ±  5%
geomean                            696.8µ         1.312m        -30.75%                ¹
¹ benchmark set differs from baseline; geomeans may not be comparable
```

## Raw output, run 1 (`-count=10`)

```
goos: darwin
goarch: arm64
pkg: anchor/cmd/it/contract
cpu: Apple M2 Max
BenchmarkHealth-12                	    1287	   1238832 ns/op	   16188 B/op	     211 allocs/op
BenchmarkHealth-12                	    2779	    845263 ns/op	   16752 B/op	     218 allocs/op
BenchmarkHealth-12                	    1988	    777137 ns/op	   16059 B/op	     210 allocs/op
BenchmarkHealth-12                	    1874	   1106720 ns/op	   16108 B/op	     210 allocs/op
BenchmarkHealth-12                	    1447	    833471 ns/op	   16083 B/op	     210 allocs/op
BenchmarkHealth-12                	    2727	    732750 ns/op	   16072 B/op	     210 allocs/op
BenchmarkHealth-12                	    1778	    622453 ns/op	   16083 B/op	     210 allocs/op
BenchmarkHealth-12                	    3457	    409755 ns/op	   16042 B/op	     210 allocs/op
BenchmarkHealth-12                	    3934	    364356 ns/op	   16056 B/op	     210 allocs/op
BenchmarkHealth-12                	    3993	    392972 ns/op	   16053 B/op	     210 allocs/op
BenchmarkReadOrganization-12      	     550	   2036028 ns/op	  197134 B/op	    1919 allocs/op
BenchmarkReadOrganization-12      	     612	   2465263 ns/op	  196458 B/op	    1920 allocs/op
BenchmarkReadOrganization-12      	     492	   2304611 ns/op	  197561 B/op	    1920 allocs/op
BenchmarkReadOrganization-12      	     358	   2917578 ns/op	  198283 B/op	    1921 allocs/op
BenchmarkReadOrganization-12      	     466	   2483077 ns/op	  197443 B/op	    1920 allocs/op
BenchmarkReadOrganization-12      	     607	   2600635 ns/op	  196719 B/op	    1920 allocs/op
BenchmarkReadOrganization-12      	     586	   2763984 ns/op	  197182 B/op	    1919 allocs/op
BenchmarkReadOrganization-12      	     363	   2912180 ns/op	  196817 B/op	    1920 allocs/op
BenchmarkReadOrganization-12      	     540	   2432219 ns/op	  197127 B/op	    1921 allocs/op
BenchmarkReadOrganization-12      	     520	   2335349 ns/op	  196334 B/op	    1918 allocs/op
BenchmarkSearchProductUsers-12    	     409	   3388030 ns/op	  256246 B/op	    2863 allocs/op
BenchmarkSearchProductUsers-12    	     442	   3038086 ns/op	  254424 B/op	    2859 allocs/op
BenchmarkSearchProductUsers-12    	     495	   2953485 ns/op	  255100 B/op	    2859 allocs/op
BenchmarkSearchProductUsers-12    	     520	   2934215 ns/op	  253685 B/op	    2859 allocs/op
BenchmarkSearchProductUsers-12    	     412	   3107788 ns/op	  253318 B/op	    2858 allocs/op
BenchmarkSearchProductUsers-12    	     307	   3441475 ns/op	  255277 B/op	    2861 allocs/op
BenchmarkSearchProductUsers-12    	     358	   3114218 ns/op	  255452 B/op	    2861 allocs/op
BenchmarkSearchProductUsers-12    	     487	   2944417 ns/op	  253698 B/op	    2858 allocs/op
BenchmarkSearchProductUsers-12    	     314	   3438775 ns/op	  253869 B/op	    2860 allocs/op
BenchmarkSearchProductUsers-12    	     402	   2896102 ns/op	  252700 B/op	    2858 allocs/op
BenchmarkCreateProductUser-12     	     247	   4622088 ns/op	  578417 B/op	    7314 allocs/op
BenchmarkCreateProductUser-12     	     211	   6205137 ns/op	 1094125 B/op	   13984 allocs/op
BenchmarkCreateProductUser-12     	     195	   5885861 ns/op	 1517787 B/op	   19875 allocs/op
BenchmarkCreateProductUser-12     	     188	   6520838 ns/op	 1939855 B/op	   25455 allocs/op
BenchmarkCreateProductUser-12     	     159	   8730110 ns/op	 2327546 B/op	   30503 allocs/op
BenchmarkCreateProductUser-12     	     150	   7385891 ns/op	 2757082 B/op	   34981 allocs/op
BenchmarkCreateProductUser-12     	     157	   9127589 ns/op	 3013711 B/op	   39424 allocs/op
BenchmarkCreateProductUser-12     	     154	   9712565 ns/op	 3343990 B/op	   43928 allocs/op
BenchmarkCreateProductUser-12     	     129	   9556791 ns/op	 3791278 B/op	   48063 allocs/op
BenchmarkCreateProductUser-12     	     129	   9612121 ns/op	 4017221 B/op	   51794 allocs/op
BenchmarkValidationError-12       	     820	   1605756 ns/op	  125186 B/op	    1290 allocs/op
BenchmarkValidationError-12       	     816	   1443519 ns/op	  124589 B/op	    1285 allocs/op
BenchmarkValidationError-12       	     865	   1437549 ns/op	  125008 B/op	    1287 allocs/op
BenchmarkValidationError-12       	     870	   1994620 ns/op	  125211 B/op	    1287 allocs/op
BenchmarkValidationError-12       	     847	   1451645 ns/op	  124433 B/op	    1285 allocs/op
BenchmarkValidationError-12       	     810	   1568056 ns/op	  125170 B/op	    1286 allocs/op
BenchmarkValidationError-12       	     813	   1483588 ns/op	  124893 B/op	    1287 allocs/op
BenchmarkValidationError-12       	     795	   1483489 ns/op	  124647 B/op	    1286 allocs/op
BenchmarkValidationError-12       	     822	   1355635 ns/op	  125317 B/op	    1286 allocs/op
BenchmarkValidationError-12       	     943	   1525552 ns/op	  124789 B/op	    1285 allocs/op
BenchmarkNotFound-12              	     550	   3023178 ns/op	  194509 B/op	    1872 allocs/op
BenchmarkNotFound-12              	     334	   3194341 ns/op	  194118 B/op	    1871 allocs/op
BenchmarkNotFound-12              	     592	   2148521 ns/op	  194003 B/op	    1870 allocs/op
BenchmarkNotFound-12              	     531	   2268084 ns/op	  194109 B/op	    1871 allocs/op
BenchmarkNotFound-12              	     499	   2218298 ns/op	  193343 B/op	    1869 allocs/op
BenchmarkNotFound-12              	     549	   2115529 ns/op	  193329 B/op	    1869 allocs/op
BenchmarkNotFound-12              	     330	   5814992 ns/op	  194849 B/op	    1875 allocs/op
BenchmarkNotFound-12              	     398	   2794099 ns/op	  193899 B/op	    1872 allocs/op
BenchmarkNotFound-12              	     472	   2172095 ns/op	  195224 B/op	    1870 allocs/op
BenchmarkNotFound-12              	     636	   1752237 ns/op	  193445 B/op	    1869 allocs/op
BenchmarkHeapAfterWarmup-12       	577616402	         2.042 ns/op	        26.00 goroutines	        13.14 heap-alloc-MiB	        22.27 heap-inuse-MiB	       0 B/op	       0 allocs/op
BenchmarkHeapAfterWarmup-12       	588646959	         2.047 ns/op	        26.00 goroutines	        13.14 heap-alloc-MiB	        22.27 heap-inuse-MiB	       0 B/op	       0 allocs/op
BenchmarkHeapAfterWarmup-12       	592930538	         2.038 ns/op	        26.00 goroutines	        13.14 heap-alloc-MiB	        22.23 heap-inuse-MiB	       0 B/op	       0 allocs/op
BenchmarkHeapAfterWarmup-12       	591365449	         2.039 ns/op	        26.00 goroutines	        13.15 heap-alloc-MiB	        22.28 heap-inuse-MiB	       0 B/op	       0 allocs/op
BenchmarkHeapAfterWarmup-12       	590857466	         2.035 ns/op	        26.00 goroutines	        13.15 heap-alloc-MiB	        22.27 heap-inuse-MiB	       0 B/op	       0 allocs/op
BenchmarkHeapAfterWarmup-12       	585809617	         2.039 ns/op	        26.00 goroutines	        13.15 heap-alloc-MiB	        22.30 heap-inuse-MiB	       0 B/op	       0 allocs/op
BenchmarkHeapAfterWarmup-12       	590706220	         2.031 ns/op	        26.00 goroutines	        13.15 heap-alloc-MiB	        22.38 heap-inuse-MiB	       0 B/op	       0 allocs/op
BenchmarkHeapAfterWarmup-12       	592630758	         2.008 ns/op	        26.00 goroutines	        13.15 heap-alloc-MiB	        22.33 heap-inuse-MiB	       0 B/op	       0 allocs/op
BenchmarkHeapAfterWarmup-12       	592003269	         2.034 ns/op	        26.00 goroutines	        13.15 heap-alloc-MiB	        22.35 heap-inuse-MiB	       0 B/op	       0 allocs/op
BenchmarkHeapAfterWarmup-12       	580483188	         2.106 ns/op	        26.00 goroutines	        13.15 heap-alloc-MiB	        22.39 heap-inuse-MiB	       0 B/op	       0 allocs/op
BenchmarkStartupToReady-12        	       1	1218909250 ns/op	        63.92 rss-ready-MiB	        68.45 rss-warm-MiB	 1469208 B/op	   16777 allocs/op
BenchmarkStartupToReady-12        	       6	 186082604 ns/op	        64.03 rss-ready-MiB	        69.16 rss-warm-MiB	  248038 B/op	    2625 allocs/op
BenchmarkStartupToReady-12        	       6	 171156972 ns/op	        64.76 rss-ready-MiB	        68.80 rss-warm-MiB	  234917 B/op	    2467 allocs/op
BenchmarkStartupToReady-12        	       6	 178417632 ns/op	        63.75 rss-ready-MiB	        68.72 rss-warm-MiB	  248820 B/op	    2634 allocs/op
BenchmarkStartupToReady-12        	       6	 176302215 ns/op	        64.67 rss-ready-MiB	        68.97 rss-warm-MiB	  241702 B/op	    2542 allocs/op
BenchmarkStartupToReady-12        	       6	 177033896 ns/op	        65.19 rss-ready-MiB	        69.22 rss-warm-MiB	  242941 B/op	    2557 allocs/op
BenchmarkStartupToReady-12        	       6	 174597035 ns/op	        63.81 rss-ready-MiB	        68.74 rss-warm-MiB	  250961 B/op	    2677 allocs/op
BenchmarkStartupToReady-12        	       6	 180115334 ns/op	        64.83 rss-ready-MiB	        69.09 rss-warm-MiB	  243074 B/op	    2568 allocs/op
BenchmarkStartupToReady-12        	       6	 177392979 ns/op	        64.28 rss-ready-MiB	        68.69 rss-warm-MiB	  233677 B/op	    2446 allocs/op
BenchmarkStartupToReady-12        	       6	 175929250 ns/op	        64.96 rss-ready-MiB	        69.15 rss-warm-MiB	  240218 B/op	    2524 allocs/op
```
