FROM registry.suse.com/bci/bci-base:15.3
RUN zypper -n install pciutils
COPY bin/pcidevices /bin/pcidevices
ENTRYPOINT ["pcidevices"]
