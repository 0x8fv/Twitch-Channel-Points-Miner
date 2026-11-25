#!/bin/sh
# The checks CI runs. The pre-push hook runs the same script, so a push that
# passes here passes there.
#
#   ci/check.sh lint    gofmt, go vet, go mod tidy
#   ci/check.sh test    the test suite
#   ci/check.sh build   every released platform, plus the Docker platforms
#   ci/check.sh all     all of the above
#
# Every check runs even after one fails, and all failures are listed at the end.
#
# CHECK_FULL=1 runs every test from scratch in random order. Without it, Go's
# test cache skips packages whose inputs did not change, which is what narrows
# pull-request feedback. Changing go.mod, go.sum, the Go version or the test
# flags reruns everything.
set -u
cd "$(dirname "$0")/.."

# One package may take this long before it has to be split up.
PACKAGE_BUDGET_SECONDS=30
RELEASE_TARGETS="windows/amd64 linux/amd64 linux/arm64 darwin/amd64 darwin/arm64"

failed=""
run() {
	name=$1
	shift
	printf '\n== %s\n' "$name"
	if "$@"; then
		printf 'ok: %s\n' "$name"
	else
		printf 'FAILED: %s\n' "$name"
		failed="$failed
  - $name"
	fi
}

check_format() {
	unformatted=$(gofmt -l cmd internal) || return 1
	if [ -n "$unformatted" ]; then
		echo "not gofmt-formatted (run: gofmt -w cmd internal):"
		echo "$unformatted"
		return 1
	fi
}

check_vet() {
	go vet ./...
}

check_tidy() {
	go mod tidy -diff
}

check_build() {
	out=$(mktemp -d) || return 1
	status=0
	for target in $RELEASE_TARGETS; do
		echo "building $target"
		GOOS=${target%/*} GOARCH=${target#*/} CGO_ENABLED=0 go build -o "$out/" ./cmd/twitch-miner || status=1
	done
	rm -rf "$out"
	return $status
}

# The race detector needs cgo and a C compiler, which CI has and a Windows
# machine usually does not.
race_flag() {
	if [ "$(go env CGO_ENABLED)" = "1" ] && { command -v gcc >/dev/null 2>&1 || command -v cc >/dev/null 2>&1; }; then
		echo "-race"
	fi
}

check_test() {
	flags=$(race_flag)
	if [ "${CHECK_FULL:-}" = "1" ]; then
		flags="$flags -count=1 -shuffle=on"
	fi
	log=$(mktemp) || return 1
	echo "go test $flags ./..."
	# shellcheck disable=SC2086
	go test $flags ./... >"$log" 2>&1
	status=$?
	cat "$log"
	report_packages "$log" || status=1
	rm -f "$log"
	return $status
}

# report_packages lists how long each package took, slowest first, and fails
# when one is over its budget.
report_packages() {
	times=$(awk -v budget="$PACKAGE_BUDGET_SECONDS" '
		($1 == "ok" || $1 == "FAIL") && $3 != "(cached)" {
			t = $3
			sub(/s$/, "", t)
			if (t + 0 == t) printf "%8.2fs  %s%s\n", t, $2, (t > budget ? "  OVER BUDGET" : "")
		}
	' "$1" | sort -rn)
	cached=$(grep -c '(cached)' "$1")
	ran=$(printf '%s' "$times" | grep -c 's  ')
	summary="$ran package(s) ran, $cached reused from the test cache
$times"
	printf '\n%s\n' "$summary"
	if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
		{
			echo '### Test packages'
			echo '```'
			echo "$summary"
			echo '```'
		} >>"$GITHUB_STEP_SUMMARY"
	fi
	case $times in
	*"OVER BUDGET"*)
		echo "a package took longer than ${PACKAGE_BUDGET_SECONDS}s: split it"
		return 1
		;;
	esac
}

case ${1:-all} in
lint)
	run "gofmt" check_format
	run "go vet" check_vet
	run "go mod tidy" check_tidy
	;;
test)
	run "tests" check_test
	;;
build)
	run "build" check_build
	;;
all)
	run "gofmt" check_format
	run "go vet" check_vet
	run "go mod tidy" check_tidy
	run "build" check_build
	run "tests" check_test
	;;
*)
	echo "usage: ci/check.sh [lint|test|build|all]" >&2
	exit 2
	;;
esac

if [ -n "$failed" ]; then
	printf '\nfailed checks:%s\n' "$failed"
	exit 1
fi
printf '\nall checks passed\n'
