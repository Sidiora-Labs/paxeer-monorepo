# These inherited Cosmos SDK Dockerfiles require an SDK-root module and simapp.
# Keep them as historical tooling; this checkout builds paxd at the root.
# Retired targets fail explicitly, including image removal, before using Docker.
all simd-env simd-dlv simd-rmi:
	@printf '%s\n' 'Retired: these inherited Cosmos SDK simd images cannot build from this repository.' >&2
	@printf '%s\n' 'sdk/go.mod, simapp and build/simd are absent; both Dockerfiles are historical tooling.' >&2
	@printf '%s\n' 'For paxd, run make -f chain.mk build from the repository root.' >&2
	@exit 1

.PHONY: all simd-env simd-dlv simd-rmi
