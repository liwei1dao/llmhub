# Root Makefile — forwards everything to app/ so you can run `make dev-up`
# from the repo root without cd-ing.

.DEFAULT_GOAL := help

# Forward any target to app/Makefile.
%:
	@$(MAKE) -C app $@

.PHONY: help
help:
	@$(MAKE) -C app help
