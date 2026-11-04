# Retired: inherited Cosmos SDK simd environment image.
# This file is a retirement notice with no executable Docker instructions.
# The old SDK-root context required sdk/go.mod, simapp and build/simd,
# which are absent from this checkout. It cannot build or run paxd.
# sdk/contrib/images/Makefile already refuses the simd-env target explicitly.
# Direct docker builds are unsupported: this notice defines no build stage.
# For paxd, run make -f chain.mk build from the repository root.
# The root go.mod requires Go 1.25.8; chain.mk produces build/paxd.
# wrapper.sh remains historical simd tooling, not a paxd entrypoint.
