FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go mod tidy && CGO_ENABLED=0 go build -o /out/controller ./cmd/controller

FROM gcr.io/distroless/static-debian12

COPY --from=build /out/controller /usr/local/bin/controller

EXPOSE 5000

HEALTHCHECK --interval=15s --timeout=5s --start-period=5s --retries=3 \
    CMD ["/usr/local/bin/controller", "healthcheck"]

CMD ["/usr/local/bin/controller"]