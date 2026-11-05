#!/usr/bin/env bash
# Coverage of a go test -coverprofile as a Markdown table: the share of
# statements covered per package and in total, generated ent code left
# out (it is most of the code and no test is meant to cover it).
#   go test -coverprofile=cover.out ./...
#   ./scripts/coverage_summary.sh cover.out "Unit tests" >> "$GITHUB_STEP_SUMMARY"
set -euo pipefail

profile=${1:?usage: coverage_summary.sh <profile> [title]}
title=${2:-Coverage}

# A profile line: <import path>/<file>.go:<start>,<end> <statements> <count>.
# A block is counted once even if the profile lists it more than once.
awk -v title="$title" '
NR == 1 || /\/ent\/generated\// { next }
{
	split($1, loc, ":")
	file = loc[1]
	pkg = file
	sub(/\/[^\/]*$/, "", pkg)
	if (!($1 in stmts)) {
		stmts[$1] = $2
		blockPkg[$1] = pkg
	}
	if ($3 > 0) covered[$1] = 1
}
END {
	for (b in stmts) {
		p = blockPkg[b]
		total[p] += stmts[b]
		all += stmts[b]
		if (b in covered) {
			hit[p] += stmts[b]
			allHit += stmts[b]
		}
	}
	printf "### %s: %.1f%% of statements\n\n", title, all ? 100 * allHit / all : 0
	print "<details><summary>By package</summary>\n"
	print "| Package | Coverage | Statements |"
	print "|---|---:|---:|"
	n = 0
	for (p in total) names[++n] = p
	# Sorted by name (insertion sort: awk has no portable sort).
	for (i = 2; i <= n; i++) {
		v = names[i]
		for (j = i - 1; j > 0 && names[j] > v; j--) names[j + 1] = names[j]
		names[j + 1] = v
	}
	for (i = 1; i <= n; i++) {
		p = names[i]
		short = p
		sub(/^github\.com\/kosatka-cms\/kosatka\//, "", short)
		printf "| %s | %.1f%% | %d |\n", short, 100 * hit[p] / total[p], total[p]
	}
	print "\n</details>\n"
}' "$profile"
