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
.PHONY: all clean test lint-makefile lint-yaml lint-folders lint-filenames start

all: clean lint build test start

APP_DIR = components/app

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

# --no-cache
build: test
	docker compose build

start: build
	docker compose up

clean:
	docker compose down --rmi all --volumes --remove-orphans
	cd $(APP_DIR) || exit && ./mvnw clean
