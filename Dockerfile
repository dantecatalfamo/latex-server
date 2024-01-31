FROM golang:1.21

WORKDIR /go/src
COPY . .
RUN make


FROM texlive/texlive

COPY --from=0 /go/src/bin/ /usr/local/bin/

RUN mkdir -p /etc/remotex /var/lib/remotex /var/db/remotex
RUN remotex-server newconfig /etc/remotex/remotex.json

VOLUME /etc/remotex
VOLUME /var/lib/remotex
VOLUME /var/db/remotex

EXPOSE 3344/tcp

ENTRYPOINT ["remotex-server"]
CMD ["server"]
