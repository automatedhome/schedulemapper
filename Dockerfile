FROM arm32v7/golang:stretch

COPY qemu-arm-static /usr/bin/
WORKDIR /go/src/github.com/automatedhome/schedulemapper
COPY . .
RUN make build

FROM arm32v7/busybox:1.30-glibc

COPY --from=0 /go/src/github.com/automatedhome/schedulemapper/schedulemapper /usr/bin/schedulemapper

ENTRYPOINT [ "/usr/bin/schedulemapper" ]
