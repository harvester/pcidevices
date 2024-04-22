# syntax=docker/dockerfile:1.7.0

FROM registry.suse.com/bci/bci-base:15.5
#awk is needed by sriov-manage utility from nvidia to perform gpu management

RUN zypper -n install pciutils which curl ethtool awk

ARG TARGETPLATFORM

RUN if [ "$TARGETPLATFORM" != "linux/amd64" ] && [ "$TARGETPLATFORM" != "linux/arm64" ]; then \
    echo "Error: Unsupported TARGETPLATFORM: $TARGETPLATFORM" && \
    exit 1; \
    fi

ENV ARCH=${TARGETPLATFORM#linux/}

RUN /sbin/update-pciids
COPY bin/pcidevices-${ARCH} /bin/pcidevices
ENTRYPOINT ["pcidevices"]
