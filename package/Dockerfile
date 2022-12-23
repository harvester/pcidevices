FROM registry.suse.com/bci/bci-base:15.3
RUN zypper -n install pciutils which curl ethtool
RUN /sbin/update-pciids
COPY bin/pcidevices /bin/pcidevices
ENTRYPOINT ["pcidevices"]
