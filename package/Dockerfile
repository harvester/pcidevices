FROM registry.suse.com/bci/bci-base:15.5
RUN zypper -n install pciutils which curl ethtool
RUN /sbin/update-pciids
COPY bin/pcidevices /bin/pcidevices
ENTRYPOINT ["pcidevices"]
