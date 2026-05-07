ARG REPOSITOI_LOCAL_ZONE_HOSTNAME
ARG PLACIDE_RELEASES_DOCKER_REPO
ARG GO_IMAGE_TAG=1.25.9-alpine
# renovate: datasource=docker depName=neufhs-docker-releases.artifactory-zci.enedis.fr/certificates
ARG CERTIFICATES_IMAGE_TAG=cert-enedis-1

FROM ${PLACIDE_RELEASES_DOCKER_REPO}.${REPOSITOI_LOCAL_ZONE_HOSTNAME}/certificates:${CERTIFICATES_IMAGE_TAG} AS certificates

FROM remote-docker-hub.${REPOSITOI_LOCAL_ZONE_HOSTNAME}/golang:${GO_IMAGE_TAG}

ARG REPOSITOI_LOCAL_ZONE_HOSTNAME

USER root

SHELL ["/bin/ash", "-eo", "pipefail", "-c"]
