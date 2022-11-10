# OPI storage gRPC to Marvell SPDK json-rpc bridge

[![Linters](https://github.com/opiproject/opi-marvell-bridge/actions/workflows/linters.yml/badge.svg)](https://github.com/opiproject/opi-marvell-bridge/actions/workflows/linters.yml)
[![tests](https://github.com/opiproject/opi-marvell-bridge/actions/workflows/go.yml/badge.svg)](https://github.com/opiproject/opi-marvell-bridge/actions/workflows/go.yml)
[![License](https://img.shields.io/github/license/opiproject/opi-marvell-bridge?style=flat-square&color=blue&label=License)](https://github.com/opiproject/opi-marvell-bridge/blob/master/LICENSE)
[![codecov](https://codecov.io/gh/opiproject/opi-marvell-bridge/branch/main/graph/badge.svg)](https://codecov.io/gh/opiproject/opi-marvell-bridge)
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
    "context"
    "fmt"
    "os"
    "plugin"
    pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
)
// from https://github.com/opiproject/opi-api/blob/main/storage/v1alpha1/gen/go/frontend_nvme_pcie.pb.go#L3204
type FrontendNvmeServiceServer interface {
    NVMeSubsystemCreate(context.Context, *NVMeSubsystemCreateRequest) (*NVMeSubsystem, error)
    NVMeSubsystemDelete(context.Context, *NVMeSubsystemDeleteRequest) (*emptypb.Empty, error)
    NVMeSubsystemUpdate(context.Context, *NVMeSubsystemUpdateRequest) (*NVMeSubsystem, error)
    NVMeSubsystemList(context.Context, *NVMeSubsystemListRequest) (*NVMeSubsystemListResponse, error)
    NVMeSubsystemGet(context.Context, *NVMeSubsystemGetRequest) (*NVMeSubsystem, error)
    NVMeSubsystemStats(context.Context, *NVMeSubsystemStatsRequest) (*NVMeSubsystemStatsResponse, error)
    NVMeControllerCreate(context.Context, *NVMeControllerCreateRequest) (*NVMeController, error)
    NVMeControllerDelete(context.Context, *NVMeControllerDeleteRequest) (*emptypb.Empty, error)
    NVMeControllerUpdate(context.Context, *NVMeControllerUpdateRequest) (*NVMeController, error)
    NVMeControllerList(context.Context, *NVMeControllerListRequest) (*NVMeControllerListResponse, error)
    NVMeControllerGet(context.Context, *NVMeControllerGetRequest) (*NVMeController, error)
    NVMeControllerStats(context.Context, *NVMeControllerStatsRequest) (*NVMeControllerStatsResponse, error)
    NVMeNamespaceCreate(context.Context, *NVMeNamespaceCreateRequest) (*NVMeNamespace, error)
    NVMeNamespaceDelete(context.Context, *NVMeNamespaceDeleteRequest) (*emptypb.Empty, error)
    NVMeNamespaceUpdate(context.Context, *NVMeNamespaceUpdateRequest) (*NVMeNamespace, error)
    NVMeNamespaceList(context.Context, *NVMeNamespaceListRequest) (*NVMeNamespaceListResponse, error)
    NVMeNamespaceGet(context.Context, *NVMeNamespaceGetRequest) (*NVMeNamespace, error)
    NVMeNamespaceStats(context.Context, *NVMeNamespaceStatsRequest) (*NVMeNamespaceStatsResponse, error)
}
func main()
    args := os.Args[1:]
    if len(args) == 2
        pluginName := args[0]
        // Load the plugin
        // 1. Search the plugins directory for a file with the same name as the pluginName
        // that was passed in as an argument and attempt to load the shared object file.
        plug, err := plugin.Open(fmt.Sprintf("plugins/%s.so", pluginName))
        if err != nil
            log.Fatal(err)
        }
        // 2. Look for an exported symbol such as a function or variable
        // in our case we expect that every plugin will have exported a single struct
        // that implements the FrontendNvmeServiceServer interface with the name "PluginFrontendNvme"
        feNvmeSymbol, err := plug.Lookup("PluginFrontendNvme")
        if err != nil
            log.Fatal(err)
        }
        // 3. Attempt to cast the symbol to the FrontendNvmeServiceServer
        // this will allow us to call the methods on the plugins if the plugin
        // implemented the required methods or fail if it does not implement it.
        var frontendNvme pb.FrontendNvmeServiceServer
        feNvme, ok := feNvmeSymbol.(pb.FrontendNvmeServiceServer)
        if !ok
            log.Fatal("Invalid feNvme type")
        }
        // 4. If everything is ok from the previous assertions, then we can proceed
        // with calling the methods on our frontendNvme interface object
        out, err := feNvme.NVMeSubsystemCreate(in)
        // OR (untested)
        s := grpc.NewServer()
        pb.RegisterFrontendNvmeServiceServer(s, &feNvme)
    }
}
```
