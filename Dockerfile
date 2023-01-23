# SPDX-License-Identifier: Apache-2.0
# Copyright (c) 2022 Dell Inc, or its subsidiaries.

FROM docker.io/library/golang:1.19.5 as builder

WORKDIR /app

# Download necessary Go modules
COPY go.mod ./
COPY go.sum ./
RUN go mod download

# build an app
COPY *.go ./
RUN go build -v -buildmode=plugin  -o /opi-marvell-bridge.so ./frontend.go ./spdk.go ./jsonrpc.go \
 && go build -v -buildmode=default -o /opi-marvell-bridge    ./main.go

# second stage to reduce image size
FROM alpine:3.17
RUN apk add --no-cache libc6-compat
COPY --from=builder /opi-marvell-bridge /
COPY --from=builder /opi-marvell-bridge.so /
COPY --from=docker.io/fullstorydev/grpcurl:v1.8.7-alpine /bin/grpcurl /usr/local/bin/
EXPOSE 50051
CMD [ "/opi-marvell-bridge", "-port=50051" ]
HEALTHCHECK CMD grpcurl -plaintext localhost:50051 list || exit 1