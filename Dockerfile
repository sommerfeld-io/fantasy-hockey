# @file Dockerfile
# @brief ...
#
# @description ...
# todo ... write description and brief (above)
# todo ... something about this being part of a bigger application stack
#
# ! THIS SHOULD MOVE TO docs
# ! NOT IN ROOT !!!!!
#
# == About the tags and versions
#
# todo ...
#
# == How to use this image
#
# todo ... reference a docker-compose stack because otherwise the app is not complete
#
# [source, bash]
# ```
# todo ...
# ```


FROM node:21.6.1-alpine3.19 AS build
LABEL maintainer="sebastian@sommerfeld.io"

RUN yarn global add @asciidoctor/core@~3.0.2 \
    && yarn global add asciidoctor-kroki@~0.18.1 \
    && yarn global add @antora/cli@3.1.7 \
    && yarn global add @antora/site-generator@3.1.7

COPY . /workspaces/fantasy-hockey
WORKDIR /workspaces/fantasy-hockey

RUN antora docs/playbook.yml --stacktrace


FROM httpd:2.4.58-alpine3.19 AS run
LABEL maintainer="sebastian@sommerfeld.io"

COPY docs/httpd.conf /usr/local/apache2/conf/httpd.conf

ARG USER=www-data
RUN chown -hR "$USER:$USER" /usr/local/apache2 \
    && chmod g-w /usr/local/apache2/conf/httpd.conf \
    && chmod g-r /etc/shadow \
    && rm /usr/local/apache2/htdocs/index.html

COPY --from=build /target/docs/public /usr/local/apache2/htdocs/docs
