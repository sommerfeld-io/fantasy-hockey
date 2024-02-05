# @file Dockerfile
# @brief ...
#
# @description ...
# todo ... write description and brief (above)
#
# == Prerequisites
#
# This image has been developed and tested with Docker version 24.0.2 on top of Ubuntu 23.10.
#
# == About the tags and versions
#
# todo ...
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
# == See also
#
# todo ... list all relevant information and links


#21-alpine3.19-jdk
FROM amazoncorretto:21-alpine3.19 as build
LABEL maintainer="sebastian@sommerfeld.io"

COPY . /app
WORKDIR /app
RUN ./mvnw clean verify


FROM ubuntu:24.04
LABEL maintainer="sebastian@sommerfeld.io"

RUN apt-get update \
    && apt-get install -y --no-install-recommends openjdk-21-jre=21.0.1+12-2~23.10 \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/*

EXPOSE 8080

COPY --from=build /app/target/app.jar /opt/fantasy-hockey/app.jar

ENTRYPOINT [ "java" ]
CMD [ "-jar", "/opt/fantasy-hockey/app.jar" ]
