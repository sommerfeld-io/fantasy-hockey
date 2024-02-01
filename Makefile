# @file Makefile
# @brief ... TODO ...
#
# @description ...
# Todo ...
#
# === Prerequisites
#
# Todo ...
#
# == How to use this Makefile
#
# Todo ...

.DEFAULT_GOAL := all
.PHONY: all clean test lint-makefile lint-yaml lint-folders lint-filenames

all: test clean

lint-makefile:
	docker run --rm --volume "$(shell pwd):/data" cytopia/checkmake:latest Makefile

lint-yaml:
	docker run --rm  $$(tty -s && echo "-it" || echo) --volume $(shell pwd):/data cytopia/yamllint:latest .

lint-folders:
	docker run --rm -i --volume "$(shell pwd):$(shell pwd)" --workdir "$(shell pwd)" sommerfeldio/folderslint:latest folderslint

lint-filenames:
	docker run --rm -i --volume "$(shell pwd):/data" --workdir "/data" lslintorg/ls-lint:1.11.2

test: lint-makefile lint-yaml lint-folders lint-filenames
	docker run --rm -i hadolint/hadolint:latest < Dockerfile.docs

# build:
# todo build docker containers but build every other thing of interest first (the app etc)

clean:
	docker compose down --rmi all --volumes --remove-orphans
