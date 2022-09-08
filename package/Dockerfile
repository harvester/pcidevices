FROM alpine:3.16
RUN apk add ebtables
RUN apk -U add pciutils
COPY bin/pcidevices /usr/bin/
CMD ["pcidevices"]