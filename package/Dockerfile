FROM gcr.io/distroless/static
COPY bin/pcidevices /bin/pcidevices
ENTRYPOINT ["pcidevices"]
# https://pci-ids.ucw.cz/v2.2/pci.ids mirror
ADD https://github.com/pciutils/pciids/raw/master/pci.ids /usr/share/hwdata/pci.ids
