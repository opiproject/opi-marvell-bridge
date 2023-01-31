// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Dell Inc, or its subsidiaries.

package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"plugin"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	port := flag.Int("port", 50051, "The server port")

	flag.Parse()
	// Load the plugin
	plug, err := plugin.Open("/opi-marvell-bridge.so")
	if err != nil {
		log.Fatal(err)
	}
	// 2. Look for an exported symbol such as a function or variable
	newServerFunc, err := plug.Lookup("NewServer")
	if err != nil {
		log.Fatal(err)
	}
	// 3. Attempt to cast the symbol to the server
	nvmeServiceServer := newServerFunc.(func() *server)()

	log.Printf("plugin server is %v", nvmeServiceServer)
	// 4. If everything is ok from the previous assertions, then we can proceed
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()

	pb.RegisterFrontendNvmeServiceServer(s, nvmeServiceServer)
	reflection.Register(s)

	log.Printf("server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
