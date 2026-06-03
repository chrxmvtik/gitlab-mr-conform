ARG REPOSITOI_LOCAL_ZONE_HOSTNAME
ARG PLACIDE_STAGES_DOCKER_REPO
ARG IMAGE_TAG=3.23.4
ARG GO_IMAGE_TAG=1.26.3-alpine

ARG PLACIDE_STAGES_DOCKER_REPO
ARG IMAGE_TAG=3.23.4
ARG GO_IMAGE_TAG=1.26.3-alpine

ARG CERTIFICATES_IMAGE_TAG=cert-enedis-1

FROM ${PLACIDE_STAGES_DOCKER_REPO}.${REPOSITOI_LOCAL_ZONE_HOSTNAME}/certificates:${CERTIFICATES_IMAGE_TAG} AS certificates
FROM ${PLACIDE_STAGES_DOCKER_REPO}.${REPOSITOI_LOCAL_ZONE_HOSTNAME}/certificates:${CERTIFICATES_IMAGE_TAG} AS certificates

FROM remote-docker-hub.${REPOSITOI_LOCAL_ZONE_HOSTNAME}/alpine:${IMAGE_TAG}
FROM remote-docker-hub.${REPOSITOI_LOCAL_ZONE_HOSTNAME}/alpine:${IMAGE_TAG}

ARG REPOSITOI_LOCAL_ZONE_HOSTNAME

USER root

SHELL ["/bin/ash", "-eo", "pipefail", "-c"]

# Ajout de l'utilitaire update-ca-certificates
WORKDIR /tmp
# hadolint ignore=DL3018
RUN alpine_version="$(cat /etc/alpine-release | awk -F "." '{print $1"."$2}')" \
    && touch repo.list \
    && repo_listing="$(wget -q --no-check-certificate -O - "https://${REPOSITOI_LOCAL_ZONE_HOSTNAME}/artifactory/remote-alpine-alpinelinux/v${alpine_version}/main/x86_64/")" \
    && ca_certificate_apk="$(echo "${repo_listing}" | grep -oE 'ca-certificates-[0-9]{8}-r[0-9]+\.apk' | awk 'NR==1')" \
    && wget -q --no-check-certificate "https://${REPOSITOI_LOCAL_ZONE_HOSTNAME}/artifactory/remote-alpine-alpinelinux/v${alpine_version}/main/x86_64/${ca_certificate_apk}" \
    && apk add --repositories-file=repo.list --allow-untrusted --no-network --no-cache "/tmp/${ca_certificate_apk}" \
    && rm /tmp/*.apk

# Ajout des certificats
WORKDIR /usr/local/share/ca-certificates
COPY --from=certificates /certs/ ./

# hadolint ignore=DL3018
RUN for f in *.cer; do mv "$f" "${f%.cer}.pem"; done \
    && update-ca-certificates --fresh \
    && alpine_version="$(cat /etc/alpine-release | awk -F "." '{print $1"."$2}')" \
    && echo "https://${REPOSITOI_LOCAL_ZONE_HOSTNAME}/artifactory/remote-alpine-alpinelinux/v${alpine_version}/main/" > /etc/apk/repositories \
    && echo "https://${REPOSITOI_LOCAL_ZONE_HOSTNAME}/artifactory/remote-alpine-alpinelinux/v${alpine_version}/community/" >> /etc/apk/repositories \
    && apk --no-cache update \
    && apk --no-cache upgrade \
    && apk --no-cache add curl tar shadow iputils jq bash git git-flow moreutils gettext ssmtp msmtp zip netcat-openbsd openssh shellcheck \
    && unlink /usr/sbin/sendmail \
    && ln -s /usr/bin/msmtp /usr/sbin/sendmail

WORKDIR /
# hadolint ignore=DL3018
RUN ln -sf /bin/bash /usr/bin/bash \
    && mkdir /logiciels/ \
    && groupadd -g 3500 webgrp && useradd -K MAIL_DIR=/dev/null -d /home/webadm -g 3500 -s /bin/bash -u 3501 webadm \
    && mkdir -p /appli/projects/ && chown -R webadm:webgrp /appli/projects \
    && mkdir -p /var/projects/ && chown -R webadm:webgrp /var/projects \
    && chown -R webadm:webgrp /logiciels/ \
    && sed -i -e 's,nobody.*,nobody:x:99:99:nobody:/:/sbin/nologin,g' /etc/passwd \
    && sed -i -e 's,nobody.*,nobody:x:99:,g' /etc/group \
    && sed -i -e 's/^root::/root:!:/' /etc/shadow \
    && usermod -a -G nobody webadm \
    && mkdir /home/webadm \
    && chown -R webadm:webgrp /home/webadm \
    && apk --update add --no-cache --virtual .build-deps  tzdata \
    && touch /etc/timezone \
    && cp /usr/share/zoneinfo/Europe/Paris /etc/localtime \
    && echo '"Europe/Paris"' > /etc/timezone

USER webadm

VOLUME /var/projects

WORKDIR /home/webadm

ENTRYPOINT ["/bin/sh", "-c", "/bin/bash"]

FROM --platform=$BUILDPLATFORM golang:1.26.3-alpine AS build

# Install corporate certificates to allow go mod download through corporate proxy
RUN apk add --no-cache ca-certificates
COPY --from=certificates /certs/ /tmp/corp-certs/
RUN for f in /tmp/corp-certs/*.cer; do cp "$f" "/tmp/corp-certs/${f%.cer}.pem" 2>/dev/null || true; done \
    && cat /tmp/corp-certs/*.pem >> /etc/ssl/certs/ca-certificates.crt \
    && rm -rf /tmp/corp-certs

# Use corporate Artifactory Go proxy
ENV GOPROXY=https://repositoi.zca.enedis.fr/artifactory/api/go/proxy-go-golang
ENV GONOSUMDB=*

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

COPY . .

# Build arguments for cross-compilation
ARG TARGETOS=linux
ARG TARGETARCH

# Build the application
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build \
    -ldflags='-w -s' \
    -o bot ./cmd/bot

# Final stage - minimal distroless image
FROM remote-docker-gcr.artifactory-zci.enedis.fr/distroless/static-debian12:nonroot

# Copy binary and configs from build stage
COPY --from=build --chown=nonroot:nonroot /app/bot /
COPY --from=build --chown=nonroot:nonroot /app/configs ./configs

# Use nonroot user (already defined in distroless image)
USER nonroot

# Run the bot application
CMD ["/bot"]
