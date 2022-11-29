# OPI storage gRPC to Marvell SPDK json-rpc bridge

[![Linters](https://github.com/opiproject/opi-marvell-bridge/actions/workflows/linters.yml/badge.svg)](https://github.com/opiproject/opi-marvell-bridge/actions/workflows/linters.yml)
[![tests](https://github.com/opiproject/opi-marvell-bridge/actions/workflows/go.yml/badge.svg)](https://github.com/opiproject/opi-marvell-bridge/actions/workflows/go.yml)
[![Docker](https://github.com/opiproject/opi-marvell-bridge/actions/workflows/docker-publish.yml/badge.svg)](https://github.com/opiproject/opi-marvell-bridge/actions/workflows/docker-publish.yml)
[![License](https://img.shields.io/github/license/opiproject/opi-marvell-bridge?style=flat-square&color=blue&label=License)](https://github.com/opiproject/opi-marvell-bridge/blob/master/LICENSE)
[![codecov](https://codecov.io/gh/opiproject/opi-marvell-bridge/branch/main/graph/badge.svg)](https://codecov.io/gh/opiproject/opi-marvell-bridge)
[![Go Report Card](https://goreportcard.com/badge/github.com/opiproject/opi-marvell-bridge)](https://goreportcard.com/report/github.com/opiproject/opi-marvell-bridge)
[![Last Release](https://img.shields.io/github/v/release/opiproject/opi-marvell-bridge?label=Latest&style=flat-square&logo=go)](https://github.com/opiproject/opi-marvell-bridge/releases)

This is a Marvell plugin to OPI storage APIs based on SPDK.

## I Want To Contribute

This project welcomes contributions and suggestions.  We are happy to have the Community involved via submission of **Issues and Pull Requests** (with substantive content or even just fixes). We are hoping for the documents, test framework, etc. to become a community process with active engagement.  PRs can be reviewed by by any number of people, and a maintainer may accept.

See [CONTRIBUTING](https://github.com/opiproject/opi/blob/main/CONTRIBUTING.md) and [GitHub Basic Process](https://github.com/opiproject/opi/blob/main/doc-github-rules.md) for more details.

## Getting started

```bash
go build -v -buildmode=plugin -o /opi-marvell-bridge.so ./...
```

 in main app:

```go
package main
import (
    "plugin"
    pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
)
func main()
    plug, err := plugin.Open("/opi-marvell-bridge.so")
    feNvmeSymbol, err := plug.Lookup("PluginFrontendNvme")
    var feNvme pb.FrontendNvmeServiceServer
    feNvme, ok := feNvmeSymbol.(pb.FrontendNvmeServiceServer)
    s := grpc.NewServer()
    pb.RegisterFrontendNvmeServiceServer(s, feNvme)
    reflection.Register(s)
}
```
