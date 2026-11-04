#!/bin/sh
# Run the complete finite accountability experiment with an identified toolchain.
set -eu
umask 077

fail() {
    printf '%s\n' "$*" >&2
    exit 1
}

: "${APALACHE_TESTS_DIR:?Set APALACHE_TESTS_DIR to the pinned apalache-tests checkout}"
: "${APALACHE_DIR:?Set APALACHE_DIR to a built Apalache checkout}"
: "${APALACHE_REVISION:?Set APALACHE_REVISION to its full lowercase Git commit}"
: "${RESULTS_ROOT:?Set RESULTS_ROOT to an existing private archive directory}"
HARNESS_REVISION=1b76b8e70549f6cd71a758b4014e7f7682d7fd44
BENCHMARK=001indinv-apalache
RUN_SCRIPT=./run-all.sh

for prerequisite in git python3 bash java mktemp; do
    command -v "$prerequisite" >/dev/null 2>&1 || fail "Missing prerequisite: $prerequisite"
done
for prerequisite in /usr/bin/time /usr/bin/timeout; do
    [ -x "$prerequisite" ] || fail "Missing prerequisite: $prerequisite (Linux GNU time/coreutils required)"
done
[ "${#APALACHE_REVISION}" -eq 40 ] || fail "APALACHE_REVISION must be a full Git commit"
case "$APALACHE_REVISION" in
    *[!0-9a-f]*) fail "APALACHE_REVISION must be lowercase hexadecimal" ;;
esac

MODEL_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd -P)
APALACHE_TESTS_DIR=$(CDPATH= cd -- "$APALACHE_TESTS_DIR" && pwd -P)
APALACHE_DIR=$(CDPATH= cd -- "$APALACHE_DIR" && pwd -P)
RESULTS_ROOT=$(CDPATH= cd -- "$RESULTS_ROOT" && pwd -P)
# The pinned generator writes unquoted paths into its generated shell scripts.
case "$MODEL_DIR:$APALACHE_TESTS_DIR:$APALACHE_DIR:$RESULTS_ROOT" in
    *[!a-zA-Z0-9_./:-]*) fail "Harness paths must contain only letters, digits, underscore, dot, slash, colon or hyphen" ;;
esac
SCRIPTS_DIR=$APALACHE_TESTS_DIR/scripts
for entry in "$APALACHE_TESTS_DIR:$HARNESS_REVISION" "$APALACHE_DIR:$APALACHE_REVISION"; do
    checkout=${entry%:*}
    revision=${entry##*:}
    [ "$(git -C "$checkout" rev-parse --show-toplevel)" = "$checkout" ] || fail "Not a checkout root: $checkout"
    [ "$(git -C "$checkout" rev-parse HEAD)" = "$revision" ] || fail "Revision mismatch: $checkout"
    checkout_status=$(git -C "$checkout" status --porcelain --untracked-files=all)
    [ -z "$checkout_status" ] || fail "Checkout must be clean (ignore build outputs explicitly): $checkout"
done
for script in mk-run.py parse-logs.py; do
    [ -r "$SCRIPTS_DIR/$script" ] || fail "Missing harness script: $SCRIPTS_DIR/$script"
done
[ -x "$APALACHE_DIR/bin/apalache-mc" ] || fail "Build the pinned Apalache checkout first"
[ -w "$RESULTS_ROOT" ] || fail "Archive is not writable: $RESULTS_ROOT"
python3 "$MODEL_DIR/experiment.py" validate-benchmarks "$MODEL_DIR/$BENCHMARK.csv"
REPOSITORY_REVISION=$(git -C "$MODEL_DIR" rev-parse HEAD)
model_status=$(git -C "$MODEL_DIR" status --porcelain --untracked-files=all -- .)
[ -z "$model_status" ] || fail "Commit the model and runner before reproducing results"

ARCHIVE=$(mktemp -d "$RESULTS_ROOT/$BENCHMARK-repo-$REPOSITORY_REVISION-harness-$HARNESS_REVISION-apalache-$APALACHE_REVISION.XXXXXX")
RUN_DIR=$ARCHIVE/runs
RESULT_CSV=$ARCHIVE/$BENCHMARK-$APALACHE_REVISION.csv
printf '%s\n' \
    "repository_revision=$REPOSITORY_REVISION" \
    "harness_revision=$HARNESS_REVISION" \
    "apalache_revision=$APALACHE_REVISION" \
    "harness_directory=$APALACHE_TESTS_DIR" \
    "apalache_directory=$APALACHE_DIR" \
    "model_directory=$MODEL_DIR" \
    "benchmark=$BENCHMARK.csv" \
    "run_script=$RUN_SCRIPT" > "$ARCHIVE/revisions.txt"
cp "$MODEL_DIR/$BENCHMARK.csv" "$ARCHIVE/$BENCHMARK.csv"
printf 'Private experiment archive: %s\n' "$ARCHIVE"

# Trace exact expanded invocations, preserve failures, and retain every raw log.
set +e
(
    set -ex
    java -version
    "$APALACHE_DIR/bin/apalache-mc" version
    python3 "$SCRIPTS_DIR/mk-run.py" "$MODEL_DIR/$BENCHMARK.csv" "$APALACHE_DIR" "$MODEL_DIR" "$RUN_DIR"
    (cd "$RUN_DIR" && bash "$RUN_SCRIPT")
    python3 "$SCRIPTS_DIR/parse-logs.py" "$RUN_DIR" --output "$RESULT_CSV"
    python3 "$MODEL_DIR/experiment.py" validate-results "$MODEL_DIR/$BENCHMARK.csv" "$RESULT_CSV"
) > "$ARCHIVE/runner.log" 2>&1
status=$?
set -e
printf 'exit=%s\n' "$status" >> "$ARCHIVE/revisions.txt"
printf 'Experiment exit: %s; private log: %s/runner.log\n' "$status" "$ARCHIVE"
exit "$status"
