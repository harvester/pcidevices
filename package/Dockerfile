FROM registry.suse.com/bci/bci-base:15.5
#awk is needed by sriov-manage utility from nvidia to perform gpu management
RUN zypper -n install pciutils which curl ethtool awk
RUN /sbin/update-pciids
COPY bin/pcidevices /bin/pcidevices
ENTRYPOINT ["pcidevices"]
