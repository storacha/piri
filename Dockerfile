FROM golang:1.25-bullseye as build

WORKDIR /go/src/piri

COPY go.* .
RUN go mod download
COPY . .

#RUN CGO_ENABLED=0 go build -o /go/bin/piri ./cmd/main.go
RUN make piri

FROM gcr.io/distroless/static-debian12
COPY --from=build /go/src/piri /usr/bin/piri

ENTRYPOINT ["/usr/bin/piri"]
