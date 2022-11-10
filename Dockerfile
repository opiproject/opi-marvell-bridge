# syntax=docker/dockerfile:1

# Alpine is chosen for its small footprint
# compared to Ubuntu
FROM docker.io/library/golang:1.19.3

WORKDIR /app

# Download necessary Go modules
COPY go.mod ./
COPY go.sum ./
RUN go mod download

# build an app
COPY *.go ./
RUN go build -v -o /opi-marvell-bridge && CGO_ENABLED=0 go test -v ./...
RUN go build -v -buildmode=plugin -o /opi-marvell-bridge.so ./frontend.go ./spdk.go ./jsonrpc.go

EXPOSE 50051
CMD [ "/opi-marvell-bridge" ]
