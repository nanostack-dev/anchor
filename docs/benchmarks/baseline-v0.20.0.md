# Baseline benchmarks: nanostack-framework v0.20.0

The reference numbers for the framework beta migration. Anchor at `4b10d222`
(origin/main), nanostack-framework v0.20.0, recorded 2026-11-08 on one
developer machine. Compare a later step with the same command on the same
machine, then `benchstat baseline-v0.20.0.txt after.txt`.

```sh
cd apps/anchor
go test -tags contract -run '^$' -bench . -benchmem -count=10 -timeout 60m ./cmd/it/contract/   # or: make contract-bench
```

Machine: Apple M2 Max (darwin/arm64), go1.27.3, Docker 29.7.2 (Docker
Desktop) for TimescaleDB 2.23.0-pg18 and Redis 7.

## Summary (median of 10, ± is the 95% confidence interval from benchstat)

| Benchmark | What it exercises | Time/op | B/op | allocs/op |
|---|---|---:|---:|---:|
| `Health` | `GET /health`: no auth, no request log, one tenant lookup | 256.9 µs ± 7% | 15.9 KiB | 205 |
| `ReadOrganization` | `GET /v1/products/{p}/organizations/{o}` with a product API key: validator, key auth (Redis cache), one row, request log | 2.02 ms ± 39% | 193 KiB | 1,944 |
| `SearchProductUsers` | `POST .../product-users/search`, a 20-item cursor page of 50 seeded users | 2.59 ms ± 7% | 245 KiB | 2,835 |
| `CreateProductUser` | `POST .../product-users`: validation, transaction, `product-events` enqueue | 6.43 ms ± 31% | 2.89 MiB ± 60% | 39.8 k ± 60% |
| `ValidationError` | the same route with an invalid email: 400 `VALIDATION_ERROR`, two log lines | 1.34 ms ± 30% | 119 KiB | 1,266 |
| `NotFound` | unknown organization: 404 `ORGANIZATION_NOT_FOUND` | 1.76 ms ± 9% | 189 KiB | 1,895 |
| `StartupToReady` | the `go build ./cmd` binary, process start to first `GET /health` 200, DB already migrated | 184.6 ms ± 2% | - | - |

| Memory | Value |
|---|---:|
| Process RSS when ready (`StartupToReady`, `ps rss`) | 75.0 MiB ± 2% |
| Process RSS after 400 warm-up requests | 78.7 MiB ± 1% |
| Test-process heap in use after 1,000 mixed requests and 2 GCs (`HeapAfterWarmup`) | 28.0 MiB (heap alloc 16.7 MiB, 26 goroutines) |

## How to read these numbers

- The endpoint benchmarks send real HTTP over loopback to the in-process
  server the golden suite boots (same fx graph as `cmd/main.go`), one request
  at a time. Logging is on (JSON to a pipe the suite drains); the suite stops
  indexing log lines while benchmarks run.
- `B/op` and `allocs/op` count everything in the test process: the HTTP
  client, the server, and the server's background workers. The client side is
  the same before and after a migration step. `CreateProductUser` enqueues a
  product event per request and the in-process events worker delivers it in
  the background, which is why its allocations and time vary so much: compare
  its medians with care.
- `ReadOrganization` and `ValidationError` are noisy on this machine (± 30-39%)
  from Docker Desktop I/O; rerun both sides back to back.
- `StartupToReady` runs the binary as a subprocess against the suite's
  containers. Its `B/op` columns count only the polling client and are left
  out above. The first start after a build was about 1.2 s (cold caches) and
  is not in these samples, because `-count=10` reuses one built binary.
- `HeapAfterWarmup` measures the test process, which also holds the
  testcontainers client and the suite; its time/op is meaningless (the
  benchmark only reports metrics).

## benchstat

```
goos: darwin
goarch: arm64
pkg: anchor/cmd/it/contract
cpu: Apple M2 Max
                      │ baseline-v0.20.0.txt │
                      │        sec/op        │
Health-12                       256.9µ ±  7%
ReadOrganization-12             2.015m ± 39%
SearchProductUsers-12           2.593m ±  7%
CreateProductUser-12            6.433m ± 31%
ValidationError-12              1.342m ± 30%
NotFound-12                     1.758m ±  9%
HeapAfterWarmup-12              2.038n ±  1%
StartupToReady-12               184.6m ±  2%
geomean                         544.0µ

                      │ baseline-v0.20.0.txt │
                      │         B/op         │
Health-12                    15.93Ki ±  1%
ReadOrganization-12          193.1Ki ±  0%
SearchProductUsers-12        245.0Ki ±  1%
CreateProductUser-12         2.889Mi ± 60%
ValidationError-12           119.1Ki ±  1%
NotFound-12                  189.0Ki ±  0%
HeapAfterWarmup-12             0.000 ±  0%
StartupToReady-12            239.8Ki ±  3%
geomean                                    ¹
¹ summaries must be >0 to compute geomean

                      │ baseline-v0.20.0.txt │
                      │      allocs/op       │
Health-12                      205.0 ±  0%
ReadOrganization-12           1.944k ±  0%
SearchProductUsers-12         2.835k ±  0%
CreateProductUser-12          39.81k ± 60%
ValidationError-12            1.266k ±  0%
NotFound-12                   1.895k ±  0%
HeapAfterWarmup-12             0.000 ±  0%
StartupToReady-12             2.593k ±  4%
geomean                                    ¹
¹ summaries must be >0 to compute geomean

                   │ baseline-v0.20.0.txt │
                   │      goroutines      │
HeapAfterWarmup-12            26.00 ± 12%

                   │ baseline-v0.20.0.txt │
                   │    heap-alloc-MiB    │
HeapAfterWarmup-12             16.69 ± 0%

                   │ baseline-v0.20.0.txt │
                   │    heap-inuse-MiB    │
HeapAfterWarmup-12             28.01 ± 0%

                  │ baseline-v0.20.0.txt │
                  │    rss-ready-MiB     │
StartupToReady-12             75.03 ± 2%

                  │ baseline-v0.20.0.txt │
                  │     rss-warm-MiB     │
StartupToReady-12             78.68 ± 1%
```

## Raw output (`-count=10`)

```
goos: darwin
goarch: arm64
pkg: anchor/cmd/it/contract
cpu: Apple M2 Max
BenchmarkHealth-12                	    4366	    256710 ns/op	   16448 B/op	     206 allocs/op
BenchmarkHealth-12                	    4598	    256605 ns/op	   16316 B/op	     205 allocs/op
BenchmarkHealth-12                	    4646	    259318 ns/op	   16330 B/op	     205 allocs/op
BenchmarkHealth-12                	    4666	    264688 ns/op	   16697 B/op	     209 allocs/op
BenchmarkHealth-12                	    4468	    257093 ns/op	   16311 B/op	     205 allocs/op
BenchmarkHealth-12                	    4598	    259205 ns/op	   16303 B/op	     205 allocs/op
BenchmarkHealth-12                	    4125	    308267 ns/op	   16297 B/op	     204 allocs/op
BenchmarkHealth-12                	    5265	    232385 ns/op	   16307 B/op	     205 allocs/op
BenchmarkHealth-12                	    5005	    238026 ns/op	   16304 B/op	     204 allocs/op
BenchmarkHealth-12                	    5076	    238320 ns/op	   16313 B/op	     204 allocs/op
BenchmarkReadOrganization-12      	     625	   1987648 ns/op	  197870 B/op	    1944 allocs/op
BenchmarkReadOrganization-12      	     636	   1938872 ns/op	  198113 B/op	    1944 allocs/op
BenchmarkReadOrganization-12      	     576	   2118157 ns/op	  197661 B/op	    1944 allocs/op
BenchmarkReadOrganization-12      	     622	   1845622 ns/op	  196864 B/op	    1942 allocs/op
BenchmarkReadOrganization-12      	     672	   2807840 ns/op	  196943 B/op	    1944 allocs/op
BenchmarkReadOrganization-12      	     544	   2023348 ns/op	  197961 B/op	    1944 allocs/op
BenchmarkReadOrganization-12      	     596	   2002873 ns/op	  197874 B/op	    1944 allocs/op
BenchmarkReadOrganization-12      	     596	   2006016 ns/op	  196950 B/op	    1943 allocs/op
BenchmarkReadOrganization-12      	     590	   2180184 ns/op	  197593 B/op	    1944 allocs/op
BenchmarkReadOrganization-12      	     534	   4082630 ns/op	  197863 B/op	    1947 allocs/op
BenchmarkSearchProductUsers-12    	     453	   2701510 ns/op	  251853 B/op	    2836 allocs/op
BenchmarkSearchProductUsers-12    	     506	   2411685 ns/op	  249974 B/op	    2833 allocs/op
BenchmarkSearchProductUsers-12    	     456	   2537239 ns/op	  251990 B/op	    2837 allocs/op
BenchmarkSearchProductUsers-12    	     450	   2745898 ns/op	  251198 B/op	    2835 allocs/op
BenchmarkSearchProductUsers-12    	     463	   2426135 ns/op	  251171 B/op	    2835 allocs/op
BenchmarkSearchProductUsers-12    	     561	   2420510 ns/op	  252437 B/op	    2837 allocs/op
BenchmarkSearchProductUsers-12    	     448	   2705674 ns/op	  250665 B/op	    2837 allocs/op
BenchmarkSearchProductUsers-12    	     523	   2319036 ns/op	  249426 B/op	    2833 allocs/op
BenchmarkSearchProductUsers-12    	     270	   3821609 ns/op	  248449 B/op	    2834 allocs/op
BenchmarkSearchProductUsers-12    	     468	   2648440 ns/op	  249359 B/op	    2834 allocs/op
BenchmarkCreateProductUser-12     	     289	   4159355 ns/op	  621450 B/op	    7865 allocs/op
BenchmarkCreateProductUser-12     	     261	   4445665 ns/op	 1222053 B/op	   15899 allocs/op
BenchmarkCreateProductUser-12     	     270	   4685594 ns/op	 1804670 B/op	   23584 allocs/op
BenchmarkCreateProductUser-12     	     231	   5277862 ns/op	 2365770 B/op	   30875 allocs/op
BenchmarkCreateProductUser-12     	     198	   6178240 ns/op	 2865873 B/op	   37081 allocs/op
BenchmarkCreateProductUser-12     	     178	   6687310 ns/op	 3193106 B/op	   42548 allocs/op
BenchmarkCreateProductUser-12     	     174	   6706378 ns/op	 3760263 B/op	   47664 allocs/op
BenchmarkCreateProductUser-12     	     176	   6986046 ns/op	 4082798 B/op	   52733 allocs/op
BenchmarkCreateProductUser-12     	     163	   7411088 ns/op	 4451289 B/op	   57660 allocs/op
BenchmarkCreateProductUser-12     	     147	   8241540 ns/op	 4931930 B/op	   62164 allocs/op
BenchmarkValidationError-12       	     778	   1505446 ns/op	  122556 B/op	    1269 allocs/op
BenchmarkValidationError-12       	     880	   1360995 ns/op	  122230 B/op	    1266 allocs/op
BenchmarkValidationError-12       	     884	   1295405 ns/op	  121691 B/op	    1265 allocs/op
BenchmarkValidationError-12       	     906	   1253453 ns/op	  121741 B/op	    1265 allocs/op
BenchmarkValidationError-12       	     994	   1738873 ns/op	  122725 B/op	    1267 allocs/op
BenchmarkValidationError-12       	     549	   2474732 ns/op	  121380 B/op	    1267 allocs/op
BenchmarkValidationError-12       	     852	   1339725 ns/op	  121608 B/op	    1265 allocs/op
BenchmarkValidationError-12       	     891	   1345089 ns/op	  121203 B/op	    1266 allocs/op
BenchmarkValidationError-12       	    1015	   1186925 ns/op	  122423 B/op	    1266 allocs/op
BenchmarkValidationError-12       	     889	   1217263 ns/op	  122649 B/op	    1266 allocs/op
BenchmarkNotFound-12              	     662	   1678118 ns/op	  193130 B/op	    1895 allocs/op
BenchmarkNotFound-12              	     710	   1669444 ns/op	  193451 B/op	    1896 allocs/op
BenchmarkNotFound-12              	     583	   1802725 ns/op	  193790 B/op	    1895 allocs/op
BenchmarkNotFound-12              	     686	   1712231 ns/op	  193554 B/op	    1895 allocs/op
BenchmarkNotFound-12              	     688	   1908486 ns/op	  193759 B/op	    1896 allocs/op
BenchmarkNotFound-12              	     639	   2124130 ns/op	  193607 B/op	    1896 allocs/op
BenchmarkNotFound-12              	     698	   1719101 ns/op	  193506 B/op	    1895 allocs/op
BenchmarkNotFound-12              	     679	   1825194 ns/op	  193567 B/op	    1895 allocs/op
BenchmarkNotFound-12              	     658	   1796045 ns/op	  193979 B/op	    1895 allocs/op
BenchmarkNotFound-12              	     716	   1685739 ns/op	  192886 B/op	    1894 allocs/op
BenchmarkHeapAfterWarmup-12       	581271315	         2.042 ns/op	        26.00 goroutines	        16.69 heap-alloc-MiB	        28.09 heap-inuse-MiB	       0 B/op	       0 allocs/op
BenchmarkHeapAfterWarmup-12       	570971114	         2.060 ns/op	        29.00 goroutines	        16.69 heap-alloc-MiB	        28.09 heap-inuse-MiB	       0 B/op	       0 allocs/op
BenchmarkHeapAfterWarmup-12       	585113254	         2.020 ns/op	        26.00 goroutines	        16.68 heap-alloc-MiB	        27.98 heap-inuse-MiB	       0 B/op	       0 allocs/op
BenchmarkHeapAfterWarmup-12       	590599501	         2.048 ns/op	        26.00 goroutines	        16.69 heap-alloc-MiB	        27.95 heap-inuse-MiB	       0 B/op	       0 allocs/op
BenchmarkHeapAfterWarmup-12       	585347056	         2.068 ns/op	        26.00 goroutines	        16.69 heap-alloc-MiB	        28.01 heap-inuse-MiB	       0 B/op	       0 allocs/op
BenchmarkHeapAfterWarmup-12       	598431236	         2.002 ns/op	        29.00 goroutines	        16.69 heap-alloc-MiB	        27.99 heap-inuse-MiB	       0 B/op	       0 allocs/op
BenchmarkHeapAfterWarmup-12       	595884301	         2.043 ns/op	        26.00 goroutines	        16.69 heap-alloc-MiB	        27.99 heap-inuse-MiB	       0 B/op	       0 allocs/op
BenchmarkHeapAfterWarmup-12       	594366272	         2.022 ns/op	        26.00 goroutines	        16.69 heap-alloc-MiB	        28.04 heap-inuse-MiB	       0 B/op	       0 allocs/op
BenchmarkHeapAfterWarmup-12       	562625601	         2.035 ns/op	        26.00 goroutines	        16.70 heap-alloc-MiB	        28.01 heap-inuse-MiB	       0 B/op	       0 allocs/op
BenchmarkHeapAfterWarmup-12       	591238705	         2.027 ns/op	        26.00 goroutines	        16.70 heap-alloc-MiB	        28.04 heap-inuse-MiB	       0 B/op	       0 allocs/op
BenchmarkStartupToReady-12        	       1	1208057875 ns/op	        72.84 rss-ready-MiB	        77.66 rss-warm-MiB	 1367544 B/op	   15591 allocs/op
BenchmarkStartupToReady-12        	       6	 182321333 ns/op	        76.17 rss-ready-MiB	        78.90 rss-warm-MiB	  244552 B/op	    2583 allocs/op
BenchmarkStartupToReady-12        	       6	 185028375 ns/op	        74.97 rss-ready-MiB	        77.88 rss-warm-MiB	  248888 B/op	    2640 allocs/op
BenchmarkStartupToReady-12        	       6	 184185348 ns/op	        74.14 rss-ready-MiB	        77.59 rss-warm-MiB	  246504 B/op	    2603 allocs/op
BenchmarkStartupToReady-12        	       6	 179523715 ns/op	        76.34 rss-ready-MiB	        79.03 rss-warm-MiB	  240021 B/op	    2538 allocs/op
BenchmarkStartupToReady-12        	       6	 180800445 ns/op	        75.08 rss-ready-MiB	        78.92 rss-warm-MiB	  242826 B/op	    2571 allocs/op
BenchmarkStartupToReady-12        	       6	 180729424 ns/op	        75.70 rss-ready-MiB	        79.28 rss-warm-MiB	  240098 B/op	    2563 allocs/op
BenchmarkStartupToReady-12        	       6	 186692896 ns/op	        74.46 rss-ready-MiB	        78.00 rss-warm-MiB	  242341 B/op	    2574 allocs/op
BenchmarkStartupToReady-12        	       6	 185204993 ns/op	        74.90 rss-ready-MiB	        78.45 rss-warm-MiB	  251904 B/op	    2691 allocs/op
BenchmarkStartupToReady-12        	       6	 188270854 ns/op	        76.29 rss-ready-MiB	        79.11 rss-warm-MiB	  246996 B/op	    2640 allocs/op
```
