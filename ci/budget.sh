#!/bin/sh
# Fails the job when it took longer than its budget, so a slow job is noticed
# the day it gets slow. JOB_STARTED is set by the job's first step.
set -eu
elapsed=$(($(date +%s) - JOB_STARTED))
line="took ${elapsed}s of a ${BUDGET_SECONDS}s budget"
echo "$line"
if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
	echo "**${GITHUB_JOB:-job}** $line" >>"$GITHUB_STEP_SUMMARY"
fi
if [ "$elapsed" -gt "$BUDGET_SECONDS" ]; then
	echo "::error::${GITHUB_JOB:-job} took ${elapsed}s, over its ${BUDGET_SECONDS}s budget. Make it faster; do not raise the budget without a measured reason."
	exit 1
fi
