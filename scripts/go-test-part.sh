#!/usr/bin/env bash
# Runs one part of a Go package's top-level tests, so one suite splits over
# several runners. Test names come from the sources (`go test -list` would run
# TestMain, which starts the containers). Fails unless every selected test ran,
# so a name the filter misses cannot pass silently.
#
# Without GO_TEST_TIMES a part takes every <parts>th test name from offset
# <part>. GO_TEST_TIMES names a file of "<test> <seconds>" lines: the tests
# then go, longest first, to the part with the least time so far (a test
# missing from the file counts 0.5 s), so the parts take about as long.
#
# Usage: scripts/go-test-part.sh <package dir> <part> <parts>
# Example: GO_TEST_TIMES=scripts/go-test-times.txt scripts/go-test-part.sh apps/anchor/cmd/it/ct 0 12
# GO_TEST_FLAGS adds flags to `go test` (CI sets -parallel).
set -euo pipefail

if [[ $# -ne 3 || ! $2 =~ ^[0-9]+$ || ! $3 =~ ^[1-9][0-9]*$ || $2 -ge $3 ]]; then
  printf 'Usage: %s <package dir> <part> <parts>\n' "$0" >&2
  exit 2
fi
dir="$1" part="$2" parts="$3"
times=""
if [[ -n ${GO_TEST_TIMES:-} ]]; then times="$(cd "$(dirname "$GO_TEST_TIMES")" && pwd)/$(basename "$GO_TEST_TIMES")"; fi
cd "$dir"
names=$(grep -hoE '^func Test[A-Za-z0-9_]+\(t \*testing\.T\)' ./*_test.go |
  sed -E 's/^func (Test[A-Za-z0-9_]+).*/\1/' | sort -u)
if [[ -n $times ]]; then
  tests=$(awk -v times="$times" '
      BEGIN { while ((getline line < times) > 0) if (line !~ /^#/) { split(line, f, " "); secs[f[1]] = f[2] } }
      { printf "%s %s\n", (($0 in secs) ? secs[$0] : 0.5), $0 }' <<< "$names" |
    LC_ALL=C sort -k1,1gr -k2,2 |
    awk -v n="$parts" -v p="$part" '
      { best = 0; for (i = 1; i < n; i++) if (load[i] < load[best]) best = i
        load[best] += $1; if (best == p) print $2 }' | paste -sd'|' -)
else
  tests=$(awk -v n="$parts" -v p="$part" 'NR % n == p' <<< "$names" | paste -sd'|' -)
fi
if [[ -z $tests ]]; then
  printf 'No tests selected in %s, part %s of %s\n' "$dir" "$part" "$parts" >&2
  exit 1
fi
read -ra extra <<< "${GO_TEST_FLAGS:-}"
json="$(mktemp)"
b="$(mktemp -d)"
SECONDS=0; go test -c -o "$b/a.test" -debug-actiongraph="$b/ag.json" ${extra[@]+"${extra[@]}"} . ; echo "MEASURE compile_and_link=${SECONDS}s"
python3 - "$b/ag.json" <<'PY'
import json,sys,re
from datetime import datetime
a=json.load(open(sys.argv[1]))
def t(s):
    m=re.match(r'(.*T\d\d:\d\d:\d\d)(\.\d+)?(.*)',s); return datetime.fromisoformat(m.group(1)+(m.group(2) or '.0')[:7].ljust(7,'0')+m.group(3))
rows=[]; n=0
for x in a:
    if x.get('TimeStart') and x.get('TimeDone') and not x.get('CacheHit') and x.get('Mode') in ('build','link','vet'):
        n+=1; rows.append(((t(x['TimeDone'])-t(x['TimeStart'])).total_seconds(),x['Mode'],x.get('Package','')))
hits=sum(1 for x in a if x.get('CacheHit'))
print('AG rebuilt %d actions, %d cache hits, sum %.1fs'%(n,hits,sum(r[0] for r in rows)))
for r in sorted(rows,reverse=True)[:12]: print('AG %.2f %s %s'%r)
PY
SECONDS=0; TEST_LOG_LEVEL=info "$b/a.test" -test.run '^$' 2>&1 | while IFS= read -r line; do printf 'TM %s %s\n' "$SECONDS" "${line:0:120}"; done | awk '!seen[substr($0,index($0," ")+1,50)]++' | grep -vE "Pull|Waiting|Verifying|Download|Extracting|[0-9a-f]{12}:" | head -40
rc=0
go run gotest.tools/gotestsum@v1.13.0 --format pkgname-and-test-fails --jsonfile "$json" -- ${extra[@]+"${extra[@]}"} -run "^($tests)\$" . || rc=$?
want=$(tr '|' '\n' <<< "$tests" | wc -l)
ran=$(grep -oE '"Action":"(pass|skip)","Package":"[^"]*","Test":"Test[^/"]*"' "$json" | wc -l)
printf 'ran %s of %s selected tests\n' "$ran" "$want"
# The slow tests, and every test's work (its time plus its direct subtests')
# as WORK lines, from which scripts/go-test-times.txt is regenerated.
printf 'top-level tests over 5 s:\n'
go run gotest.tools/gotestsum@v1.13.0 tool slowest --jsonfile "$json" --threshold 5s | awk '$2 !~ /\//' || true
if [[ -n ${GITHUB_ACTIONS:-} ]]; then printf '::group::work per test\n'; fi
command -v jq > /dev/null && jq -rs '[.[] | select(.Action == "pass" and .Test)] | group_by(.Test | split("/")[0])[] |
  "WORK \(.[0].Test | split("/")[0]) \(map(select(.Test | split("/") | length <= 2) | .Elapsed) | add)"' "$json" || true
if [[ -n ${GITHUB_ACTIONS:-} ]]; then printf '::endgroup::\n'; fi
if [[ $ran -ne $want ]]; then exit 1; fi
exit "$rc"
