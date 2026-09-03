FROM sommerfeldio/devcontainer:0.28.1
LABEL maintainer="sebastian@sommerfeld.io"

## Development container for Go projects.
##
## Installs:
## - Go toolchain (go compiler, formatter, test runner, module support)
## - gopls (official Go language server used by IDEs and editors)
## - dlv (Delve debugger for Go applications and tests)
## - build-essential (required for CGO-based builds and native dependencies)
## - ca-certificates (required for secure HTTPS downloads)
##
## The Go version can be configured via the GO_VERSION build argument.

ARG GO_VERSION=1.26.4
ARG ARCH=linux-amd64

USER root

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        curl \
        git \
        build-essential \
        ca-certificates \
    && rm -rf /var/lib/apt/lists/*

RUN curl -fsSL "https://go.dev/dl/go${GO_VERSION}.${ARCH}.tar.gz" -o /tmp/go.tar.gz \
    && rm -rf /usr/local/go \
    && tar -C /usr/local -xzf /tmp/go.tar.gz \
    && rm /tmp/go.tar.gz

ENV PATH="/usr/local/go/bin:${PATH}"

RUN go install golang.org/x/tools/gopls@latest \
    && go install github.com/go-delve/delve/cmd/dlv@latest

ENV PATH="${PATH}:/root/go/bin"

USER vscode
