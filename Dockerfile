FROM --platform=$BUILDPLATFORM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG TARGETOS TARGETARCH VERSION=dev
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o /out/zyxel-1250-exporter ./cmd/zyxel-1250-exporter

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/zyxel-1250-exporter /zyxel-1250-exporter
EXPOSE 9092
ENTRYPOINT ["/zyxel-1250-exporter"]
