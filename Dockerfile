FROM golang:1.17.6 as builder
LABEL org.opencontainers.image.source=https://github.com/tailify/hetzner-cloud-controller-manager
WORKDIR /maschine-controller/src
COPY ./go.mod .
COPY ./go.sum .
RUN go mod download
ADD . .
RUN CGO_ENABLED=0 go build -o hcloud-cloud-controller-manager  .

FROM alpine:3.12
RUN apk add --no-cache ca-certificates bash
USER root
RUN mkdir -p /home
WORKDIR /home

COPY --from=builder /maschine-controller/src/hcloud-cloud-controller-manager /bin/hcloud-cloud-controller-manager

ENTRYPOINT ["/bin/bash", "-c"]