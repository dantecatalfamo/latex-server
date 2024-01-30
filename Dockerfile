FROM golang:1.21

WORKDIR /go/src
COPY . .
RUN make


FROM texlive/texlive

COPY --from=0 /go/src/bin/ /usr/local/bin/
