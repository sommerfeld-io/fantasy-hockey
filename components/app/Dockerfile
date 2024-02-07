# @file Dockerfile
# @brief The main Docker image for the Fantasy Hockey app.
#
# @description The provided Dockerfile utilizes a multistage build approach to build and
# run the Fantasy Hockey app. In the first stage, it employs the Maven image to build the
# application's JAR file.
#
# In the second stage, it runs the application on port 8080. This multistage approach
# optimizes the Docker image size by discarding unnecessary build dependencies after the
# first stage, resulting in a more efficient final image for running the application.
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
# todo ... at least update the image names below
#
# [source, bash]
# ```
# docker build -t local/fantasy-hockey-app:dev .
# docker run --rm -it -p 8080:8080 local/fantasy-hockey-app:dev
# ```
#
# To avoid downloading every Maven artifact with every build, the local Maven repository
# is mounted into the container.
# todo ... https://github.com/sommerfeld-io/fantasy-hockey/issues/9
#
# == Eclipse Temurin instead of JDK
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
#
# == See also
#
# * https://spring.io/guides/topicals/spring-boot-docker/
# * https://www.docker.com/blog/9-tips-for-containerizing-your-spring-boot-code


FROM eclipse-temurin:21.0.2_13-jdk-jammy as builder
LABEL maintainer="sebastian@sommerfeld.io"

RUN addgroup fantasy-hockey && adduser --ingroup fantasy-hockey --disabled-password fantasy-hockey
USER fantasy-hockey

WORKDIR /opt/app
COPY .mvn/ .mvn
COPY mvnw ./
COPY pom.xml ./
RUN ./mvnw dependency:go-offline
COPY ./src ./src
RUN ./mvnw clean install


FROM eclipse-temurin:21.0.2_13-jre-jammy
LABEL maintainer="sebastian@sommerfeld.io"

RUN addgroup fantasy-hockey && adduser --ingroup fantasy-hockey --disabled-password fantasy-hockey
USER fantasy-hockey

EXPOSE 8080
COPY --from=builder /opt/app/target/app.jar /opt/app/app.jar
ENTRYPOINT ["java", "-jar", "/opt/app/app.jar" ]
