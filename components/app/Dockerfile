# @file Dockerfile
# @brief The main Docker image for the Fantasy Hockey app.
#
# @description This Docker image is part of a broader set of images that collectively constitute
# the Fantasy Hockey App, a private NHL fantasy game.
#
# The underlying Dockerfile utilizes a multistage build approach to build and run the Fantasy
# Hockey App. In the first stage, it employs the Maven image to build the application's JAR file.
#
# In the second stage, it runs the application on port 8080. This multistage approach optimizes
# the Docker image size by discarding unnecessary build dependencies after the first stage,
# resulting in a more efficient final image for running the application.
#
# == Prerequisites
#
# This image has been developed with Docker version 24.0.8-1 inside the
# ``mcr.microsoft.com/vscode/devcontainers/java:21-bookworm`` Dev Container image.
#
# == About the tags and versions
#
# todo ... https://github.com/sommerfeld-io/fantasy-hockey/issues/8
#
# == How to use this image
#
# [source, bash]
# ```
# docker build -t local/fantasy-hockey-app:dev .
# docker run --rm -it -p 8080:8080 local/fantasy-hockey-app:dev
# ```
#
# == Eclipse Temurin Base Image instead of JDK
#
# On the OpenJDK link:https://hub.docker.com/_/openjdk[Docker Hub] page, you’ll find a list of
# recommended Docker Official Images that you should use while building Java applications.
# The upstream OpenJDK image is deprecated and no longer provides a JRE, so no official JRE
# images are produced. The official OpenJDK images just contain “vanilla” builds of the
# OpenJDK provided by link:https://jdk.java.net[Oracle] or the
# link:https://github.com/docker-library/openjdk/issues/320#issuecomment-494050246[relevant project lead].
#
# One of the most popular official images with a build-worthy JDK is Eclipse Temurin. The
# Eclipse Temurin project provides code and processes that support the building of runtime
# binaries and associated technologies. These are high performance, enterprise-caliber, and
# cross-platform.


FROM eclipse-temurin:21.0.2_13-jdk-jammy as builder
LABEL maintainer="sebastian@sommerfeld.io"

ARG MVN_OPTS="--no-transfer-progress -Dstyle.color=always"

RUN addgroup fantasy-hockey && adduser --ingroup fantasy-hockey --disabled-password fantasy-hockey
USER fantasy-hockey

WORKDIR /opt/app
COPY .mvn/ .mvn
COPY mvnw ./
COPY pom.xml ./
RUN ./mvnw $MVN_OPTS dependency:go-offline
COPY ./src ./src
RUN ./mvnw $MVN_OPTS clean install


FROM eclipse-temurin:21.0.2_13-jre-jammy
LABEL maintainer="sebastian@sommerfeld.io"
LABEL org.opencontainers.image.source="https://github.com/sommerfeld-io/fantasy-hockey"
LABEL org.opencontainers.image.description "The Fantasy Hockey App Runtime"

RUN addgroup fantasy-hockey && adduser --ingroup fantasy-hockey --disabled-password fantasy-hockey
USER fantasy-hockey

EXPOSE 8080
COPY --from=builder /opt/app/target/app.jar /opt/app/app.jar
ENTRYPOINT ["java", "-jar", "/opt/app/app.jar" ]
