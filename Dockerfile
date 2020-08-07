FROM scratch

ARG ARCH=amd64
COPY ./dockron-linux-${ARCH} /dockron

ENTRYPOINT [ "/dockron" ]
