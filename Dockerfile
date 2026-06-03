FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go mod tidy && CGO_ENABLED=0 go build -o /out/controller ./cmd/controller

FROM gcr.io/distroless/static-debian12

COPY --from=build /out/controller /usr/local/bin/controller

EXPOSE 5001

CMD ["/usr/local/bin/controller"]