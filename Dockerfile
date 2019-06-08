ARG REPO=library
FROM ${REPO}/busybox:latest
WORKDIR /root/

ARG ARCH=amd64
COPY ./dockron-linux-${ARCH} ./dockron

CMD [ "./dockron" ]
