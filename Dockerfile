FROM --platform=$BUILDPLATFORM golang:1.26.4-alpine3.24 AS build
LABEL maintainer="sebastian@sommerfeld.io"

ARG TARGETOS
ARG TARGETARCH

RUN apk add --no-cache curl git \
    && sh -c "$(curl --location https://taskfile.dev/install.sh)" -- -d -b /usr/local/bin

COPY .git /workspaces/fantasy-hockey/.git
WORKDIR /workspaces/fantasy-hockey/src
COPY src /workspaces/fantasy-hockey/src

## see src/taskfile.yml
RUN task build




FROM alpine:3.24.0 AS run
LABEL maintainer="sebastian@sommerfeld.io"

ARG USER_NAME=fantasy-hockey
ARG USER_ID=1000
ARG GROUP_ID=1000

RUN addgroup -g ${GROUP_ID} ${USER_NAME} \
    && adduser -D -u ${USER_ID} -G ${USER_NAME} -h /opt/fantasy-hockey ${USER_NAME} \
    && chown -R ${USER_NAME}:${USER_NAME} /opt/fantasy-hockey

WORKDIR /opt/fantasy-hockey
COPY --from=build /workspaces/fantasy-hockey/src/fantasy-hockey ./fantasy-hockey

USER ${USER_NAME}

CMD ["./fantasy-hockey"]
