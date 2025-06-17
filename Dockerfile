FROM golang:1.24-bullseye as build

WORKDIR /go/src/piri

COPY go.* .
RUN go mod download
COPY . .

RUN CGO_ENABLED=0 go build -o /go/bin/piri ./cmd/storage
RUN CGO_ENABLED=0 go build -o /go/bin/do-piri ./cmd/do

FROM gcr.io/distroless/static-debian12
COPY --from=build /go/bin/piri /usr/bin/
COPY --from=build /go/bin/do-piri /usr/bin/

ENTRYPOINT ["/usr/bin/piri"]
