// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Dell Inc, or its subsidiaries.
// Copyright (C) 2022 Marvell International Ltd.

package main

import (
	"bytes"
	"context"
	"fmt"
	"google.golang.org/protobuf/proto"
	"log"
	"net"
	"os"
	"reflect"
	"strings"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/emptypb"

	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
)

func dialer() func(context.Context, string) (net.Conn, error) {
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	pb.RegisterFrontendNvmeServiceServer(server, &PluginFrontendNvme)

	go func() {
		if err := server.Serve(listener); err != nil {
			log.Fatal(err)
		}
	}()

	return func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}
}

func spdkMockServer(l net.Listener, toSend []string) {
	for _, spdk := range toSend {
		fd, err := l.Accept()
		if err != nil {
			log.Fatal("accept error:", err)
		}
		log.Printf("SPDK mockup server: client connected [%s]", fd.RemoteAddr().Network())
		log.Printf("SPDK ID [%d]", rpcID)

		buf := make([]byte, 512)
		nr, err := fd.Read(buf)
		if err != nil {
			return
		}

		data := buf[0:nr]
		if strings.Contains(spdk, "%") {
			spdk = fmt.Sprintf(spdk, rpcID)
		}

		log.Printf("SPDK mockup server: got : %s", string(data))
		log.Printf("SPDK mockup server: snd : %s", string(spdk))

		_, err = fd.Write([]byte(string(spdk)))
		if err != nil {
			log.Fatal("Write: ", err)
		}
		err = fd.(*net.UnixConn).CloseWrite()
		if err != nil {
			log.Fatal(err)
		}
	}
}

func TestFrontEnd_CreateNVMeSubsystem(t *testing.T) {
	spec := &pb.NVMeSubsystemSpec{
		Id:           &pc.ObjectKey{Value: "subsystem-test"},
		Nqn:          "nqn.2022-09.io.spdk:opi3",
		SerialNumber: "OpiSerialNumber",
		ModelNumber:  "OpiModelNumber",
	}
	tests := []struct {
		name    string
		in      *pb.NVMeSubsystem
		out     *pb.NVMeSubsystem
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		{
			"valid request with invalid SPDK response",
			&pb.NVMeSubsystem{
				Spec: spec,
			},
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result": {"status": 1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not create NQN: %v", "nqn.2022-09.io.spdk:opi3"),
			true,
		},
		{
			"valid request with empty SPDK response",
			&pb.NVMeSubsystem{
				Spec: spec,
			},
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_create_subsystem: %v", "EOF"),
			true,
		},
		{
			"valid request with ID mismatch SPDK response",
			&pb.NVMeSubsystem{
				Spec: spec,
			},
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_create_subsystem: %v", "json response ID mismatch"),
			true,
		},
		{
			"valid request with error code from SPDK response",
			&pb.NVMeSubsystem{
				Spec: spec,
			},
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_create_subsystem: %v", "json response error: myopierr"),
			true,
		},
		{
			"valid request with invalid SPDK version response",
			&pb.NVMeSubsystem{
				Spec: spec,
			},
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0}}`, `{"jsonrpc":"2.0","id":%d,"result":{"status":1,"sdk_version":"11.22.06","nvm_version":"1.3","num_pcie_domains":1,"num_pfs_per_domain":1,"num_vfs_per_pf":16,"total_ioq_per_pf":128,"max_ioq_per_pf":128,"max_ioq_per_vf":128,"max_subsystems":16,"max_ns_per_subsys":8,"max_ctrlr_per_subsys":16}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not get FW version for NQN create request: %v", "nqn.2022-09.io.spdk:opi3"),
			true,
		},
		{
			"valid request with valid SPDK response",
			&pb.NVMeSubsystem{
				Spec: spec,
			},
			&pb.NVMeSubsystem{
				Spec: spec,
				Status: &pb.NVMeSubsystemStatus{
					FirmwareRevision: "11.22.06",
				},
			},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0}}`, `{"jsonrpc":"2.0","id":%d,"result":{"status":0,"sdk_version":"11.22.06","nvm_version":"1.3","num_pcie_domains":1,"num_pfs_per_domain":1,"num_vfs_per_pf":16,"total_ioq_per_pf":128,"max_ioq_per_pf":128,"max_ioq_per_vf":128,"max_subsystems":16,"max_ns_per_subsys":8,"max_ctrlr_per_subsys":16}}`},
			codes.OK,
			"",
			true,
		},
	}

	// start GRPC mockup server
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(dialer()))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	client := pb.NewFrontendNvmeServiceClient(conn)

	// start SPDK mockup server
	if err := os.RemoveAll(*rpcSock); err != nil {
		log.Fatal(err)
	}
	ln, err := net.Listen("unix", *rpcSock)
	if err != nil {
		log.Fatal("listen error:", err)
	}
	defer ln.Close()

	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.start {
				go spdkMockServer(ln, tt.spdk)
			}
			request := &pb.CreateNVMeSubsystemRequest{NvMeSubsystem: tt.in}
			response, err := client.CreateNVMeSubsystem(ctx, request)
			if response != nil {
				// Marshall the request and response, so we can just compare the contained data
				mtt, _ := proto.Marshal(tt.out.Spec)
				mResponse, _ := proto.Marshal(response.Spec)

				// Compare the marshalled messages
				if !bytes.Equal(mtt, mResponse) {
					t.Error("response: expected", tt.out.GetSpec(), "received", response.GetSpec())
				}
				if !reflect.DeepEqual(response.Status, tt.out.Status) {
					t.Error("response: expected", tt.out.GetStatus(), "received", response.GetStatus())
				}
			}

			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestFrontEnd_UpdateNVMeSubsystem(t *testing.T) {
	tests := []struct {
		name    string
		in      *pb.NVMeSubsystem
		out     *pb.NVMeSubsystem
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		{
			"unimplemented method",
			&pb.NVMeSubsystem{},
			nil,
			codes.Unimplemented,
			fmt.Sprintf("%v method is not implemented", "UpdateNVMeSubsystem"),
			false,
		},
	}

	// start GRPC mockup server
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(dialer()))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	client := pb.NewFrontendNvmeServiceClient(conn)

	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := &pb.UpdateNVMeSubsystemRequest{NvMeSubsystem: tt.in}
			response, err := client.UpdateNVMeSubsystem(ctx, request)
			if response != nil {
				t.Error("response: expected", codes.Unimplemented, "received", response)
			}

			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestFrontEnd_ListNVMeSubsystem(t *testing.T) {
	tests := []struct {
		name    string
		out     []*pb.NVMeSubsystem
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		{
			"valid request with invalid SPDK response",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not list %v", "subsystems"),
			true,
		},
		{
			"valid request with empty SPDK response",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_get_subsys_list: %v", "EOF"),
			true,
		},
		{
			"valid request with ID mismatch SPDK response",
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_get_subsys_list: %v", "json response ID mismatch"),
			true,
		},
		{
			"valid request with error code from SPDK response",
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_get_subsys_list: %v", "json response error: myopierr"),
			true,
		},
		{
			"valid request with valid SPDK response",
			[]*pb.NVMeSubsystem{
				{Spec: &pb.NVMeSubsystemSpec{Nqn: "nqn.2022-09.io.spdk:opi1"}},
				{Spec: &pb.NVMeSubsystemSpec{Nqn: "nqn.2022-09.io.spdk:opi2"}},
				{Spec: &pb.NVMeSubsystemSpec{Nqn: "nqn.2022-09.io.spdk:opi3"}},
			},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0, "subsys_list": [{"subnqn": "nqn.2022-09.io.spdk:opi1"},{"subnqn": "nqn.2022-09.io.spdk:opi2"},{"subnqn": "nqn.2022-09.io.spdk:opi3"}]}}`},
			codes.OK,
			"",
			true,
		},
	}

	// start GRPC mockup server
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(dialer()))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	client := pb.NewFrontendNvmeServiceClient(conn)

	// start SPDK mockup server
	if err := os.RemoveAll(*rpcSock); err != nil {
		log.Fatal(err)
	}
	ln, err := net.Listen("unix", *rpcSock)
	if err != nil {
		log.Fatal("listen error:", err)
	}
	defer ln.Close()

	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.start {
				go spdkMockServer(ln, tt.spdk)
			}
			request := &pb.ListNVMeSubsystemsRequest{}
			response, err := client.ListNVMeSubsystems(ctx, request)
			if response != nil {
				if !reflect.DeepEqual(response.NvMeSubsystems, tt.out) {
					t.Error("response: expected", tt.out, "received", response.NvMeSubsystems)
				}
			}

			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestFrontEnd_GetNVMeSubsystem(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		out     *pb.NVMeSubsystem
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		{
			"valid request with invalid SPDK response",
			"subsystem-test",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not list NQN: %v", "nqn.2022-09.io.spdk:opi3"),
			true,
		},
		{
			"valid request with empty SPDK response",
			"subsystem-test",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_get_subsys_list: %v", "EOF"),
			true,
		},
		{
			"valid request with ID mismatch SPDK response",
			"subsystem-test",
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_get_subsys_list: %v", "json response ID mismatch"),
			true,
		},
		{
			"valid request with error code from SPDK response",
			"subsystem-test",
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_get_subsys_list: %v", "json response error: myopierr"),
			true,
		},
		{
			"valid request with valid SPDK response",
			"subsystem-test",
			&pb.NVMeSubsystem{
				Spec:   &pb.NVMeSubsystemSpec{Nqn: "nqn.2022-09.io.spdk:opi3"},
				Status: &pb.NVMeSubsystemStatus{FirmwareRevision: "TBD"},
			},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0, "subsys_list": [{"subnqn": "nqn.2022-09.io.spdk:opi1"},{"subnqn": "nqn.2022-09.io.spdk:opi2"},{"subnqn": "nqn.2022-09.io.spdk:opi3"}]}}`},
			codes.OK,
			"",
			true,
		},
		{
			"valid request with unknown key",
			"unknown-subsystem-id",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("unable to find key %v", "unknown-subsystem-id"),
			false,
		},
	}

	// start GRPC mockup server
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(dialer()))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	client := pb.NewFrontendNvmeServiceClient(conn)

	// start SPDK mockup server
	if err := os.RemoveAll(*rpcSock); err != nil {
		log.Fatal(err)
	}
	ln, err := net.Listen("unix", *rpcSock)
	if err != nil {
		log.Fatal("listen error:", err)
	}
	defer ln.Close()

	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.start {
				go spdkMockServer(ln, tt.spdk)
			}
			request := &pb.GetNVMeSubsystemRequest{Name: tt.in}
			response, err := client.GetNVMeSubsystem(ctx, request)
			if response != nil {
				if !reflect.DeepEqual(response.Spec, tt.out.Spec) {
					t.Error("response: expected", tt.out.GetSpec(), "received", response.GetSpec())
				}
				if !reflect.DeepEqual(response.Status, tt.out.Status) {
					t.Error("response: expected", tt.out.GetStatus(), "received", response.GetStatus())
				}
			}

			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestFrontEnd_NVMeSubsystemStats(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		out     *pb.VolumeStats
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		{
			"valid request with invalid SPDK response",
			"subsystem-test",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not stats NQN: %v", "nqn.2022-09.io.spdk:opi3"),
			true,
		},
		{
			"valid request with invalid marshal SPDK response",
			"subsystem-test",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_get_info: %v", "json: cannot unmarshal array into Go struct field .result of type main.MrvlNvmGetSubsysInfoResult"),
			true,
		},
		{
			"valid request with empty SPDK response",
			"subsystem-test",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_get_info: %v", "EOF"),
			true,
		},
		{
			"valid request with ID mismatch SPDK response",
			"subsystem-test",
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_get_info: %v", "json response ID mismatch"),
			true,
		},
		{
			"valid request with error code from SPDK response",
			"subsystem-test",
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_get_info: %v", "json response error: myopierr"),
			true,
		},
		{
			"valid request with valid SPDK response",
			"subsystem-test",
			&pb.VolumeStats{
				ReadOpsCount:  -1,
				WriteOpsCount: -1,
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":{"status":0,"subsys_list":[{"subnqn":"nqn.2014-08.org.nvmexpress.discovery","mn":"OCTEON NVME 0.0.1","sn":"OCTNVME0000000000002","max_namespaces":16,"min_ctrlr_id":1,"max_ctrlr_id":8,"num_ns":2,"num_total_ctrlr":2,"num_active_ctrlr":2,"ns_list":[{"ns_instance_id":1,"bdev":"bdev01","ctrlr_id_list":[{"ctrlr_id":1},{"ctrlr_id":2}]},{"ns_instance_id":1,"bdev":"bdev02","ctrlr_id_list":[{"ctrlr_id":3}]}]}]}}`},
			codes.OK,
			"",
			true,
		},
		{
			"valid request with unknown key",
			"unknown-subsystem-id",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("unable to find key %v", "unknown-subsystem-id"),
			false,
		},
	}

	// start GRPC mockup server
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(dialer()))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	client := pb.NewFrontendNvmeServiceClient(conn)

	// start SPDK mockup server
	if err := os.RemoveAll(*rpcSock); err != nil {
		log.Fatal(err)
	}
	ln, err := net.Listen("unix", *rpcSock)
	if err != nil {
		log.Fatal("listen error:", err)
	}
	defer ln.Close()

	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.start {
				go spdkMockServer(ln, tt.spdk)
			}
			request := &pb.NVMeSubsystemStatsRequest{SubsystemId: &pc.ObjectKey{Value: tt.in}}
			response, err := client.NVMeSubsystemStats(ctx, request)
			if response != nil {
				if !reflect.DeepEqual(response.Stats, tt.out) {
					t.Error("response: expected", tt.out, "received", response.Stats)
				}
			}

			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestFrontEnd_CreateNVMeController(t *testing.T) {
	spec := &pb.NVMeControllerSpec{
		Id:               &pc.ObjectKey{Value: "controller-test"},
		SubsystemId:      &pc.ObjectKey{Value: "subsystem-test"},
		PcieId:           &pb.PciEndpoint{PhysicalFunction: 1, VirtualFunction: 2, PortId: 3},
		NvmeControllerId: 1,
		MaxNsq:           5,
		MaxNcq:           6,
		Sqes:             7,
		Cqes:             8,
	}
	tests := []struct {
		name    string
		in      *pb.NVMeController
		out     *pb.NVMeController
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		{
			"valid request with invalid SPDK response",
			&pb.NVMeController{
				Spec: spec,
			},
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1, "cntlid": -1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not create CTRL: %v", "controller-test"),
			true,
		},
		{
			"valid request with empty SPDK response",
			&pb.NVMeController{
				Spec: spec,
			},
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_create_ctrlr: %v", "EOF"),
			true,
		},
		{
			"valid request with ID mismatch SPDK response",
			&pb.NVMeController{
				Spec: spec,
			},
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1, "cntlid": 17}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_create_ctrlr: %v", "json response ID mismatch"),
			true,
		},
		{
			"valid request with error code from SPDK response",
			&pb.NVMeController{
				Spec: spec,
			},
			nil,
			[]string{`{"id":%d,"error":{"code":-32602,"message":"Invalid parameters"}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_create_ctrlr: %v", "json response error: Invalid parameters"),
			true,
		},
		{
			"valid request with valid SPDK response",
			&pb.NVMeController{
				Spec: &pb.NVMeControllerSpec{
					Id:               &pc.ObjectKey{Value: "controller-test"},
					SubsystemId:      &pc.ObjectKey{Value: "subsystem-test"},
					PcieId:           &pb.PciEndpoint{PhysicalFunction: 1, VirtualFunction: 2, PortId: 3},
					NvmeControllerId: 17,
					MaxNsq:           5,
					MaxNcq:           6,
					Sqes:             7,
					Cqes:             8,
				},
			},
			&pb.NVMeController{
				Spec: &pb.NVMeControllerSpec{
					Id:          &pc.ObjectKey{Value: "controller-test"},
					SubsystemId: &pc.ObjectKey{Value: "subsystem-test"},
					PcieId:      &pb.PciEndpoint{PhysicalFunction: 1, VirtualFunction: 2, PortId: 3},
					MaxNsq:      5,
					MaxNcq:      6,
					Sqes:        7,
					Cqes:        8,
				},
				Status: &pb.NVMeControllerStatus{Active: true},
			},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0, "cntlid": 17}}`},
			codes.OK,
			"",
			true,
		},
	}

	// start GRPC mockup server
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(dialer()))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	client := pb.NewFrontendNvmeServiceClient(conn)

	// start SPDK mockup server
	if err := os.RemoveAll(*rpcSock); err != nil {
		log.Fatal(err)
	}
	ln, err := net.Listen("unix", *rpcSock)
	if err != nil {
		log.Fatal("listen error:", err)
	}
	defer ln.Close()

	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.start {
				go spdkMockServer(ln, tt.spdk)
			}
			request := &pb.CreateNVMeControllerRequest{NvMeController: tt.in}
			response, err := client.CreateNVMeController(ctx, request)
			if response != nil {
				// Marshall the request and response, so we can just compare the contained data
				mtt, _ := proto.Marshal(tt.out.Spec)
				mResponse, _ := proto.Marshal(response.Spec)

				// Compare the marshalled messages
				if !bytes.Equal(mtt, mResponse) {
					t.Error("response: expected", tt.out.GetSpec(), "received", response.GetSpec())
				}
				if !reflect.DeepEqual(response.Status, tt.out.Status) {
					t.Error("response: expected", tt.out.GetStatus(), "received", response.GetStatus())
				}
			}

			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestFrontEnd_UpdateNVMeController(t *testing.T) {
	spec := &pb.NVMeControllerSpec{
		Id:               &pc.ObjectKey{Value: "controller-test"},
		SubsystemId:      &pc.ObjectKey{Value: "subsystem-test"},
		PcieId:           &pb.PciEndpoint{PhysicalFunction: 1, VirtualFunction: 2, PortId: 3},
		NvmeControllerId: 1,
		MaxNsq:           5,
		MaxNcq:           6,
		Sqes:             7,
		Cqes:             8,
	}
	tests := []struct {
		name    string
		in      *pb.NVMeController
		out     *pb.NVMeController
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		{
			"valid request with invalid SPDK response",
			&pb.NVMeController{
				Spec: spec,
			},
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1, "cntlid": -1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not update CTRL: %v", "controller-test"),
			true,
		},
		{
			"valid request with empty SPDK response",
			&pb.NVMeController{
				Spec: spec,
			},
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_update_ctrlr: %v", "EOF"),
			true,
		},
		{
			"valid request with ID mismatch SPDK response",
			&pb.NVMeController{
				Spec: spec,
			},
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1, "cntlid": 17}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_update_ctrlr: %v", "json response ID mismatch"),
			true,
		},
		{
			"valid request with error code from SPDK response",
			&pb.NVMeController{
				Spec: spec,
			},
			nil,
			[]string{`{"id":%d,"error":{"code":-32602,"message":"Invalid parameters"}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_update_ctrlr: %v", "json response error: Invalid parameters"),
			true,
		},
		{
			"valid request with valid SPDK response",
			&pb.NVMeController{
				Spec: &pb.NVMeControllerSpec{
					Id:               &pc.ObjectKey{Value: "controller-test"},
					SubsystemId:      &pc.ObjectKey{Value: "subsystem-test"},
					PcieId:           &pb.PciEndpoint{PhysicalFunction: 1, VirtualFunction: 2, PortId: 3},
					NvmeControllerId: 17,
					MaxNsq:           5,
					MaxNcq:           6,
					Sqes:             7,
					Cqes:             8,
				},
			},
			&pb.NVMeController{
				Spec: &pb.NVMeControllerSpec{
					Id:          &pc.ObjectKey{Value: "controller-test"},
					SubsystemId: &pc.ObjectKey{Value: "subsystem-test"},
					PcieId:      &pb.PciEndpoint{PhysicalFunction: 1, VirtualFunction: 2, PortId: 3},
					MaxNsq:      5,
					MaxNcq:      6,
					Sqes:        7,
					Cqes:        8,
				},
				Status: &pb.NVMeControllerStatus{Active: true},
			},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0, "cntlid": 17}}`},
			codes.OK,
			"",
			true,
		},
	}

	// start GRPC mockup server
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(dialer()))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	client := pb.NewFrontendNvmeServiceClient(conn)

	// start SPDK mockup server
	if err := os.RemoveAll(*rpcSock); err != nil {
		log.Fatal(err)
	}
	ln, err := net.Listen("unix", *rpcSock)
	if err != nil {
		log.Fatal("listen error:", err)
	}
	defer ln.Close()

	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.start {
				go spdkMockServer(ln, tt.spdk)
			}
			request := &pb.UpdateNVMeControllerRequest{NvMeController: tt.in}
			response, err := client.UpdateNVMeController(ctx, request)
			if response != nil {
				// Marshall the request and response, so we can just compare the contained data
				mtt, _ := proto.Marshal(tt.out.Spec)
				mResponse, _ := proto.Marshal(response.Spec)

				// Compare the marshalled messages
				if !bytes.Equal(mtt, mResponse) {
					t.Error("response: expected", tt.out.GetSpec(), "received", response.GetSpec())
				}
				if !reflect.DeepEqual(response.Status, tt.out.Status) {
					t.Error("response: expected", tt.out.GetStatus(), "received", response.GetStatus())
				}
			}

			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestFrontEnd_ListNVMeControllers(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		out     []*pb.NVMeController
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		{
			"valid request with invalid SPDK response",
			"subsystem-test",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1, "ctrlr_id_list": []}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not list CTRLs: %v", "subsystem-test"),
			true,
		},
		{
			"valid request with empty SPDK response",
			"subsystem-test",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_get_ctrlr_list: %v", "EOF"),
			true,
		},
		{
			"valid request with ID mismatch SPDK response",
			"subsystem-test",
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1, "ctrlr_id_list": []}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_get_ctrlr_list: %v", "json response ID mismatch"),
			true,
		},
		{
			"valid request with error code from SPDK response",
			"subsystem-test",
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":{"status": 1, "ctrlr_id_list": []}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_get_ctrlr_list: %v", "json response error: myopierr"),
			true,
		},
		{
			"valid request with valid SPDK response",
			"subsystem-test",
			[]*pb.NVMeController{
				{
					Spec: &pb.NVMeControllerSpec{
						NvmeControllerId: 1,
					},
				},
				{
					Spec: &pb.NVMeControllerSpec{
						NvmeControllerId: 2,
					},
				},
				{
					Spec: &pb.NVMeControllerSpec{
						NvmeControllerId: 3,
					},
				},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"error":{"code":0,"message":""},"result":{"status":0,"ctrlr_id_list":[{"ctrlr_id":1},{"ctrlr_id":2},{"ctrlr_id":3}]}}`},
			codes.OK,
			"",
			true,
		},
		// {
		// 	"valid request with unknown key",
		// 	"unknown-subsystem-id",
		// 	nil,
		// 	[]string{""},
		// 	codes.Unknown,
		// 	fmt.Sprintf("unable to find key %v", "unknown-subsystem-id"),
		// 	false,
		// },
	}

	// start GRPC mockup server
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(dialer()))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	client := pb.NewFrontendNvmeServiceClient(conn)

	// start SPDK mockup server
	if err := os.RemoveAll(*rpcSock); err != nil {
		log.Fatal(err)
	}
	ln, err := net.Listen("unix", *rpcSock)
	if err != nil {
		log.Fatal("listen error:", err)
	}
	defer ln.Close()

	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.start {
				go spdkMockServer(ln, tt.spdk)
			}
			request := &pb.ListNVMeControllersRequest{Parent: tt.in}
			response, err := client.ListNVMeControllers(ctx, request)
			if response != nil {
				if !reflect.DeepEqual(response.NvMeControllers, tt.out) {
					t.Error("response: expected", tt.out, "received", response.NvMeControllers)
				}
			}

			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestFrontEnd_GetNVMeController(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		out     *pb.NVMeController
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		{
			"valid request with invalid SPDK response",
			"controller-test",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status":1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not get CTRL: %v", "controller-test"),
			true,
		},
		{
			"valid request with empty SPDK response",
			"controller-test",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ctrlr_get_info: %v", "EOF"),
			true,
		},
		{
			"valid request with ID mismatch SPDK response",
			"controller-test",
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status":1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ctrlr_get_info: %v", "json response ID mismatch"),
			true,
		},
		{
			"valid request with error code from SPDK response",
			"controller-test",
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":{"status":1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ctrlr_get_info: %v", "json response error: myopierr"),
			true,
		},
		{
			"valid request with valid SPDK response",
			"controller-test",
			&pb.NVMeController{
				Spec: &pb.NVMeControllerSpec{
					Id: &pc.ObjectKey{Value: "controller-test"},
				},
				Status: &pb.NVMeControllerStatus{Active: true},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":{"status":0,"pcie_domain_id":1,"pf_id":1,"vf_id":1,"ctrlr_id":1,"max_nsq":4,"max_ncq":4,"mqes":2048,"ieee_oui":"005043","cmic":6,"nn":16,"active_ns_count":4,"active_nsq":2,"active_ncq":2,"mdts":9,"sqes":6,"cqes":4}}`},
			codes.OK,
			"",
			true,
		},
		{
			"valid request with unknown key",
			"unknown-subsystem-id",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("error finding controller %v", "unknown-subsystem-id"),
			false,
		},
	}

	// start GRPC mockup server
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(dialer()))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	client := pb.NewFrontendNvmeServiceClient(conn)

	// start SPDK mockup server
	if err := os.RemoveAll(*rpcSock); err != nil {
		log.Fatal(err)
	}
	ln, err := net.Listen("unix", *rpcSock)
	if err != nil {
		log.Fatal("listen error:", err)
	}
	defer ln.Close()

	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.start {
				go spdkMockServer(ln, tt.spdk)
			}
			request := &pb.GetNVMeControllerRequest{Name: tt.in}
			response, err := client.GetNVMeController(ctx, request)
			if response != nil {
				if !reflect.DeepEqual(response.Spec, tt.out.Spec) {
					t.Error("response: expected", tt.out.GetSpec(), "received", response.GetSpec())
				}
				if !reflect.DeepEqual(response.Status, tt.out.Status) {
					t.Error("response: expected", tt.out.GetStatus(), "received", response.GetStatus())
				}
			}

			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestFrontEnd_NVMeControllerStats(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		out     *pb.VolumeStats
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		{
			"valid request with invalid SPDK response",
			"controller-test",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not stats CTRL: %v", "controller-test"),
			true,
		},
		{
			"valid request with invalid marshal SPDK response",
			"controller-test",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ctrlr_get_stats: %v", "json: cannot unmarshal array into Go struct field .result of type main.MrvlNvmGetCtrlrStatsResult"),
			true,
		},
		{
			"valid request with empty SPDK response",
			"controller-test",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ctrlr_get_stats: %v", "EOF"),
			true,
		},
		{
			"valid request with ID mismatch SPDK response",
			"controller-test",
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ctrlr_get_stats: %v", "json response ID mismatch"),
			true,
		},
		{
			"valid request with error code from SPDK response",
			"controller-test",
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ctrlr_get_stats: %v", "json response error: myopierr"),
			true,
		},
		{
			"valid request with valid SPDK response",
			"controller-test",
			&pb.VolumeStats{
				ReadBytesCount:    5,
				ReadOpsCount:      4,
				WriteBytesCount:   7,
				WriteOpsCount:     6,
				ReadLatencyTicks:  9,
				WriteLatencyTicks: 10,
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":{"status":0,"num_admin_cmds":1,"num_admin_cmd_errors":2,"num_async_events":3,"num_read_cmds":4,"num_read_bytes":5,"num_write_cmds":6,"num_write_bytes":7,"num_errors":8,"total_read_latency_in_us":9,"total_write_latency_in_us":10,"Stats_time_window_in_us":11}}`},
			codes.OK,
			"",
			true,
		},
		{
			"valid request with unknown key",
			"unknown-controller-id",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("error finding controller %v", "unknown-controller-id"),
			false,
		},
	}

	// start GRPC mockup server
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(dialer()))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	client := pb.NewFrontendNvmeServiceClient(conn)

	// start SPDK mockup server
	if err := os.RemoveAll(*rpcSock); err != nil {
		log.Fatal(err)
	}
	ln, err := net.Listen("unix", *rpcSock)
	if err != nil {
		log.Fatal("listen error:", err)
	}
	defer ln.Close()

	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.start {
				go spdkMockServer(ln, tt.spdk)
			}
			request := &pb.NVMeControllerStatsRequest{Id: &pc.ObjectKey{Value: tt.in}}
			response, err := client.NVMeControllerStats(ctx, request)
			if response != nil {
				if !reflect.DeepEqual(response.Stats, tt.out) {
					t.Error("response: expected", tt.out, "received", response.Stats)
				}
			}

			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestFrontEnd_CreateNVMeNamespace(t *testing.T) {
	spec := &pb.NVMeNamespaceSpec{
		Id:          &pc.ObjectKey{Value: "namespace-test"},
		SubsystemId: &pc.ObjectKey{Value: "subsystem-test"},
		HostNsid:    0,
		VolumeId:    &pc.ObjectKey{Value: "Malloc1"},
		Uuid:        &pc.Uuid{Value: "1b4e28ba-2fa1-11d2-883f-b9a761bde3fb"},
		Nguid:       "1b4e28ba-2fa1-11d2-883f-b9a761bde3fb",
		Eui64:       1967554867335598546,
	}
	tests := []struct {
		name    string
		in      *pb.NVMeNamespace
		out     *pb.NVMeNamespace
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		{
			"valid request with invalid SPDK response",
			&pb.NVMeNamespace{
				Spec: spec,
			},
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not create NS: %v", "namespace-test"),
			true,
		},
		{
			"valid request with empty SPDK response",
			&pb.NVMeNamespace{
				Spec: spec,
			},
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_alloc_ns: %v", "EOF"),
			true,
		},
		{
			"valid request with ID mismatch SPDK response",
			&pb.NVMeNamespace{
				Spec: spec,
			},
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_alloc_ns: %v", "json response ID mismatch"),
			true,
		},
		{
			"valid request with error code from SPDK response",
			&pb.NVMeNamespace{
				Spec: spec,
			},
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_alloc_ns: %v", "json response error: myopierr"),
			true,
		},
		{
			"valid request with valid SPDK response",
			&pb.NVMeNamespace{
				Spec: &pb.NVMeNamespaceSpec{
					Id:          &pc.ObjectKey{Value: "namespace-test"},
					SubsystemId: &pc.ObjectKey{Value: "subsystem-test"},
					HostNsid:    22,
					VolumeId:    &pc.ObjectKey{Value: "Malloc1"},
					Uuid:        &pc.Uuid{Value: "1b4e28ba-2fa1-11d2-883f-b9a761bde3fb"},
					Nguid:       "1b4e28ba-2fa1-11d2-883f-b9a761bde3fb",
					Eui64:       1967554867335598546,
				},
			},
			&pb.NVMeNamespace{
				Spec: &pb.NVMeNamespaceSpec{
					Id:          &pc.ObjectKey{Value: "namespace-test"},
					SubsystemId: &pc.ObjectKey{Value: "subsystem-test"},
					HostNsid:    22,
					VolumeId:    &pc.ObjectKey{Value: "Malloc1"},
					Uuid:        &pc.Uuid{Value: "1b4e28ba-2fa1-11d2-883f-b9a761bde3fb"},
					Nguid:       "1b4e28ba-2fa1-11d2-883f-b9a761bde3fb",
					Eui64:       1967554867335598546,
				},
				Status: &pb.NVMeNamespaceStatus{
					PciState:     2,
					PciOperState: 1,
				},
			},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0, "ns_instance_id": 17}}`, `{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0}}`},
			codes.OK,
			"",
			true,
		},
		{
			"valid request with invalid SPDK second attach response",
			&pb.NVMeNamespace{
				Spec: spec,
			},
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0, "ns_instance_id": 17}}`, `{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not attach NS: %v", "namespace-test"),
			true,
		},
	}

	// start GRPC mockup server
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(dialer()))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	client := pb.NewFrontendNvmeServiceClient(conn)

	// start SPDK mockup server
	if err := os.RemoveAll(*rpcSock); err != nil {
		log.Fatal(err)
	}
	ln, err := net.Listen("unix", *rpcSock)
	if err != nil {
		log.Fatal("listen error:", err)
	}
	defer ln.Close()

	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.start {
				go spdkMockServer(ln, tt.spdk)
			}
			request := &pb.CreateNVMeNamespaceRequest{NvMeNamespace: tt.in}
			response, err := client.CreateNVMeNamespace(ctx, request)
			if response != nil {
				// Marshall the request and response, so we can just compare the contained data
				mtt, _ := proto.Marshal(tt.out.Spec)
				mResponse, _ := proto.Marshal(response.Spec)

				// Compare the marshalled messages
				if !bytes.Equal(mtt, mResponse) {
					t.Error("response: expected", tt.out.GetSpec(), "received", response.GetSpec())
				}
				if !reflect.DeepEqual(response.Status, tt.out.Status) {
					t.Error("response: expected", tt.out.GetStatus(), "received", response.GetStatus())
				}
			}

			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestFrontEnd_UpdateNVMeNamespace(t *testing.T) {
	tests := []struct {
		name    string
		in      *pb.NVMeNamespace
		out     *pb.NVMeNamespace
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		{
			"unimplemented method",
			&pb.NVMeNamespace{},
			nil,
			codes.Unimplemented,
			fmt.Sprintf("%v method is not implemented", "UpdateNVMeNamespace"),
			false,
		},
	}

	// start GRPC mockup server
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(dialer()))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	client := pb.NewFrontendNvmeServiceClient(conn)

	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := &pb.UpdateNVMeNamespaceRequest{NvMeNamespace: tt.in}
			response, err := client.UpdateNVMeNamespace(ctx, request)
			if response != nil {
				t.Error("response: expected", codes.Unimplemented, "received", response)
			}

			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestFrontEnd_ListNVMeNamespaces(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		out     []*pb.NVMeNamespace
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		{
			"valid request with invalid SPDK response",
			"subsystem-test",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status":1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not list NS: %v", "subsystem-test"),
			true,
		},
		{
			"valid request with invalid marshal SPDK response",
			"subsystem-test",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_get_ns_list: %v", "json: cannot unmarshal array into Go struct field .result of type main.MrvlNvmSubsysGetNsListResult"),
			true,
		},
		{
			"valid request with empty SPDK response",
			"subsystem-test",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_get_ns_list: %v", "EOF"),
			true,
		},
		{
			"valid request with ID mismatch SPDK response",
			"subsystem-test",
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status":1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_get_ns_list: %v", "json response ID mismatch"),
			true,
		},
		{
			"valid request with error code from SPDK response",
			"subsystem-test",
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_get_ns_list: %v", "json response error: myopierr"),
			true,
		},
		{
			"valid request with valid SPDK response",
			"subsystem-test",
			[]*pb.NVMeNamespace{
				{
					Spec: &pb.NVMeNamespaceSpec{
						HostNsid: 11,
					},
				},
				{
					Spec: &pb.NVMeNamespaceSpec{
						HostNsid: 12,
					},
				},
				{
					Spec: &pb.NVMeNamespaceSpec{
						HostNsid: 13,
					},
				},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":{"status":0,"ns_list":[{"ns_instance_id":11,"bdev":"bdev01","ctrlr_id_list":[{"ctrlr_id":1}]},{"ns_instance_id":12,"bdev":"bdev02","ctrlr_id_list":[]},{"ns_instance_id":13,"bdev":"bdev03","ctrlr_id_list":[]}]}}`},
			codes.OK,
			"",
			true,
		},
		{
			"valid request with unknown key",
			"unknown-namespace-id",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("unable to find key %v", "unknown-namespace-id"),
			false,
		},
	}

	// start GRPC mockup server
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(dialer()))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	client := pb.NewFrontendNvmeServiceClient(conn)

	// start SPDK mockup server
	if err := os.RemoveAll(*rpcSock); err != nil {
		log.Fatal(err)
	}
	ln, err := net.Listen("unix", *rpcSock)
	if err != nil {
		log.Fatal("listen error:", err)
	}
	defer ln.Close()

	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.start {
				go spdkMockServer(ln, tt.spdk)
			}
			request := &pb.ListNVMeNamespacesRequest{Parent: tt.in}
			response, err := client.ListNVMeNamespaces(ctx, request)
			if response != nil {
				if !reflect.DeepEqual(response.NvMeNamespaces, tt.out) {
					t.Error("response: expected", tt.out, "received", response.NvMeNamespaces)
				}
			}

			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestFrontEnd_GetNVMeNamespace(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		out     *pb.NVMeNamespace
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		{
			"valid request with invalid SPDK response",
			"namespace-test",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status":1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not get NS: %v", "namespace-test"),
			true,
		},
		{
			"valid request with invalid marshal SPDK response",
			"namespace-test",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ns_get_info: %v", "json: cannot unmarshal array into Go struct field .result of type main.MrvlNvmGetNsInfoResult"),
			true,
		},
		{
			"valid request with empty SPDK response",
			"namespace-test",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ns_get_info: %v", "EOF"),
			true,
		},
		{
			"valid request with ID mismatch SPDK response",
			"namespace-test",
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status":1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ns_get_info: %v", "json response ID mismatch"),
			true,
		},
		{
			"valid request with error code from SPDK response",
			"namespace-test",
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ns_get_info: %v", "json response error: myopierr"),
			true,
		},
		{
			"valid request with valid SPDK response",
			"namespace-test",
			&pb.NVMeNamespace{
				Spec: &pb.NVMeNamespaceSpec{
					Id:    &pc.ObjectKey{Value: "namespace-test"},
					Nguid: "0x25f9cbc45d0f976fb9c1a14ff5aed4b0",
				},
				Status: &pb.NVMeNamespaceStatus{
					PciState:     2,
					PciOperState: 1,
				},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":{"status":0,"nguid":"0x25f9cbc45d0f976fb9c1a14ff5aed4b0","eui64":"0xa7632f80702e4242","uuid":"0xb35633240b77073b8b4ebda571120dfb","nmic":1,"bdev":"bdev01","num_ctrlrs":1,"ctrlr_id_list":[{"ctrlr_id":1}]}}`},
			codes.OK,
			"",
			true,
		},
		{
			"valid request with unknown key",
			"unknown-namespace-id",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("unable to find key %v", "unknown-namespace-id"),
			false,
		},
	}

	// start GRPC mockup server
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(dialer()))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	client := pb.NewFrontendNvmeServiceClient(conn)

	// start SPDK mockup server
	if err := os.RemoveAll(*rpcSock); err != nil {
		log.Fatal(err)
	}
	ln, err := net.Listen("unix", *rpcSock)
	if err != nil {
		log.Fatal("listen error:", err)
	}
	defer ln.Close()

	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.start {
				go spdkMockServer(ln, tt.spdk)
			}
			request := &pb.GetNVMeNamespaceRequest{Name: tt.in}
			response, err := client.GetNVMeNamespace(ctx, request)
			if response != nil {
				if !reflect.DeepEqual(response.Spec, tt.out.Spec) {
					t.Error("response: expected", tt.out.GetSpec(), "received", response.GetSpec())
				}
				if !reflect.DeepEqual(response.Status, tt.out.Status) {
					t.Error("response: expected", tt.out.GetStatus(), "received", response.GetStatus())
				}
			}

			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestFrontEnd_NVMeNamespaceStats(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		out     *pb.VolumeStats
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		{
			"valid request with invalid SPDK response",
			"namespace-test",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not stats NS: %v", "namespace-test"),
			true,
		},
		{
			"valid request with invalid marshal SPDK response",
			"namespace-test",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ns_get_stats: %v", "json: cannot unmarshal array into Go struct field .result of type main.MrvlNvmGetNsStatsResult"),
			true,
		},
		{
			"valid request with empty SPDK response",
			"namespace-test",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ns_get_stats: %v", "EOF"),
			true,
		},
		{
			"valid request with ID mismatch SPDK response",
			"namespace-test",
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ns_get_stats: %v", "json response ID mismatch"),
			true,
		},
		{
			"valid request with error code from SPDK response",
			"namespace-test",
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ns_get_stats: %v", "json response error: myopierr"),
			true,
		},
		{
			"valid request with valid SPDK response",
			"namespace-test",
			&pb.VolumeStats{
				ReadBytesCount:    2,
				ReadOpsCount:      1,
				WriteBytesCount:   4,
				WriteOpsCount:     3,
				ReadLatencyTicks:  6,
				WriteLatencyTicks: 7,
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":{"status":0,"num_read_cmds":1,"num_read_bytes":2,"num_write_cmds":3,"num_write_bytes":4,"num_errors":5,"total_read_latency_in_us":6,"total_write_latency_in_us":7,"Stats_time_window_in_us":8}}`},
			codes.OK,
			"",
			true,
		},
		{
			"valid request with unknown key",
			"unknown-namespace-id",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("unable to find key %v", "unknown-namespace-id"),
			false,
		},
	}

	// start GRPC mockup server
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(dialer()))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	client := pb.NewFrontendNvmeServiceClient(conn)

	// start SPDK mockup server
	if err := os.RemoveAll(*rpcSock); err != nil {
		log.Fatal(err)
	}
	ln, err := net.Listen("unix", *rpcSock)
	if err != nil {
		log.Fatal("listen error:", err)
	}
	defer ln.Close()

	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.start {
				go spdkMockServer(ln, tt.spdk)
			}
			request := &pb.NVMeNamespaceStatsRequest{NamespaceId: &pc.ObjectKey{Value: tt.in}}
			response, err := client.NVMeNamespaceStats(ctx, request)
			if response != nil {
				if !reflect.DeepEqual(response.Stats, tt.out) {
					t.Error("response: expected", tt.out, "received", response.Stats)
				}
			}

			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}

func TestFrontEnd_DeleteNVMeNamespace(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		out     *emptypb.Empty
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		{
			"valid request with invalid SPDK response",
			"namespace-test",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not detach NS: %v", "namespace-test"),
			true,
		},
		{
			"valid request with invalid SPDK second response",
			"namespace-test",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0}}`, `{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not delete NS: %v", "namespace-test"),
			true,
		},
		{
			"valid request with empty SPDK response",
			"namespace-test",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ctrlr_detach_ns: %v", "EOF"),
			true,
		},
		{
			"valid request with ID mismatch SPDK response",
			"namespace-test",
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ctrlr_detach_ns: %v", "json response ID mismatch"),
			true,
		},
		{
			"valid request with error code from SPDK response",
			"namespace-test",
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ctrlr_detach_ns: %v", "json response error: myopierr"),
			true,
		},
		{
			"valid request with valid SPDK response",
			"namespace-test",
			&emptypb.Empty{},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0}}`, `{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0}}`},
			codes.OK,
			"",
			true,
		},
		{
			"valid request with unknown key",
			"unknown-namespace-id",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("unable to find key %v", "unknown-namespace-id"),
			false,
		},
	}

	// start GRPC mockup server
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(dialer()))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	client := pb.NewFrontendNvmeServiceClient(conn)

	// start SPDK mockup server
	if err := os.RemoveAll(*rpcSock); err != nil {
		log.Fatal(err)
	}
	ln, err := net.Listen("unix", *rpcSock)
	if err != nil {
		log.Fatal("listen error:", err)
	}
	defer ln.Close()

	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.start {
				go spdkMockServer(ln, tt.spdk)
			}
			request := &pb.DeleteNVMeNamespaceRequest{Name: tt.in}
			response, err := client.DeleteNVMeNamespace(ctx, request)
			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
			if reflect.TypeOf(response) != reflect.TypeOf(tt.out) {
				t.Error("response: expected", reflect.TypeOf(tt.out), "received", reflect.TypeOf(response))
			}
		})
	}
}

func TestFrontEnd_DeleteNVMeController(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		out     *emptypb.Empty
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		{
			"valid request with invalid SPDK response",
			"controller-test",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not delete CTRL: %v", "controller-test"),
			true,
		},
		{
			"valid request with empty SPDK response",
			"controller-test",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_remove_ctrlr: %v", "EOF"),
			true,
		},
		{
			"valid request with ID mismatch SPDK response",
			"controller-test",
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_remove_ctrlr: %v", "json response ID mismatch"),
			true,
		},
		{
			"valid request with error code from SPDK response",
			"controller-test",
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_remove_ctrlr: %v", "json response error: myopierr"),
			true,
		},
		{
			"valid request with valid SPDK response",
			"controller-test",
			&emptypb.Empty{},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0}}`},
			codes.OK,
			"",
			true,
		},
		{
			"valid request with unknown key",
			"unknown-controller-id",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("error finding controller %v", "unknown-controller-id"),
			false,
		},
	}

	// start GRPC mockup server
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(dialer()))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	client := pb.NewFrontendNvmeServiceClient(conn)

	// start SPDK mockup server
	if err := os.RemoveAll(*rpcSock); err != nil {
		log.Fatal(err)
	}
	ln, err := net.Listen("unix", *rpcSock)
	if err != nil {
		log.Fatal("listen error:", err)
	}
	defer ln.Close()

	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.start {
				go spdkMockServer(ln, tt.spdk)
			}
			request := &pb.DeleteNVMeControllerRequest{Name: tt.in}
			response, err := client.DeleteNVMeController(ctx, request)
			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
			if reflect.TypeOf(response) != reflect.TypeOf(tt.out) {
				t.Error("response: expected", reflect.TypeOf(tt.out), "received", reflect.TypeOf(response))
			}
		})
	}
}

func TestFrontEnd_DeleteNVMeSubsystem(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		out     *emptypb.Empty
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		{
			"valid request with invalid SPDK response",
			"subsystem-test",
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not delete NQN: %v", "nqn.2022-09.io.spdk:opi3"),
			true,
		},
		{
			"valid request with empty SPDK response",
			"subsystem-test",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_deletesubsystem: %v", "EOF"),
			true,
		},
		{
			"valid request with ID mismatch SPDK response",
			"subsystem-test",
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_deletesubsystem: %v", "json response ID mismatch"),
			true,
		},
		{
			"valid request with error code from SPDK response",
			"subsystem-test",
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_deletesubsystem: %v", "json response error: myopierr"),
			true,
		},
		{
			"valid request with valid SPDK response",
			"subsystem-test",
			&emptypb.Empty{},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0}}`},
			codes.OK,
			"",
			true,
		},
		{
			"valid request with unknown key",
			"unknown-subsystem-id",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("unable to find key %v", "unknown-subsystem-id"),
			false,
		},
	}

	// start GRPC mockup server
	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(dialer()))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()
	client := pb.NewFrontendNvmeServiceClient(conn)

	// start SPDK mockup server
	if err := os.RemoveAll(*rpcSock); err != nil {
		log.Fatal(err)
	}
	ln, err := net.Listen("unix", *rpcSock)
	if err != nil {
		log.Fatal("listen error:", err)
	}
	defer ln.Close()

	// run tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.start {
				go spdkMockServer(ln, tt.spdk)
			}
			request := &pb.DeleteNVMeSubsystemRequest{Name: tt.in}
			response, err := client.DeleteNVMeSubsystem(ctx, request)
			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", codes.InvalidArgument, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
			if reflect.TypeOf(response) != reflect.TypeOf(tt.out) {
				t.Error("response: expected", reflect.TypeOf(tt.out), "received", reflect.TypeOf(response))
			}
		})
	}
}
