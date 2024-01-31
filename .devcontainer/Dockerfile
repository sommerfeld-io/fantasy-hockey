# @file Dockerfile
# @brief A devcontainer image tailored specifically for the project.
#
# @description This Dockerfile represents the development environment container tailored
# specifically for the project. It serves as the foundation for encapsulating all necessary
# dependencies within the image, ensuring a consistent and reproducible environment across
# various workstations.
#
# === Prerequisites
#
# Having Visual Studio Code (VSCode) and the Dev Container plugin installed are
# essential prerequisites for this development environment. This devcontainer has
# been tested with Docker version 24.0.7 on top of Ubuntu 23.10.
#
# == How to use this image
#
# The dev container configuration is either located under ``.devcontainer/devcontainer.json``
# or stored as a ``.devcontainer.json`` file (note the dot-prefix) in the root of your project.
#
# You can use an image as a starting point for your devcontainer by pulling images from a
# container registry. To use this devcontainer image, add the following lines to your
# devcontainer config file.
#
# This ``.devcontainer/devcontainer.json`` config allows the use of docker containers
# inside the devcontainer and initializes ``pre-commit`` once the container is created.


FROM mcr.microsoft.com/vscode/devcontainers/base:debian-11
LABEL maintainer="sebastian@sommerfeld.io"

ARG USER=vscode

SHELL ["/bin/bash", "-o", "pipefail", "-c"]
RUN chsh --shell /bin/bash "$USER" \
    && echo "alias ls='ls -a'" >> "/home/$USER/.bashrc" \
    && echo "alias ll='ls -alF'" >> "/home/$USER/.bashrc" \
    && apt-get update \
    && apt-get install -y --no-install-recommends python3-pip=20.3.4-4+deb11u1 \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/* \
    && pip install --no-cache-dir pre-commit==3.3.3
