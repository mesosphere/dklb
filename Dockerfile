# Build the "dklb" binary taking the "vendor/" directory into account.
FROM golang:1.12.4 AS builder
ARG VERSION
ENV GOFLAGS="-mod=vendor"
WORKDIR /src
COPY . .
RUN make build

# Copy the "dklb" binary from the "builder" container.
FROM gcr.io/distroless/static:a4fd5de337e31911aeee2ad5248284cebeb6a6f4
LABEL name=mesosphere/dklb
ARG VERSION
LABEL version=${VERSION}
COPY --from=builder /src/build/dklb /dklb
ENV CLUSTER_NAME ""
ENV POD_NAME ""
ENV POD_NAMESPACE ""
EXPOSE 10250
CMD ["/dklb", "-h"]
