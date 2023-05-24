// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2022 Marvell International Ltd.
// Copyright (C) 2023 Intel Corporation

// Package frontend implememnts the FrontEnd APIs (host facing) of the storage Server
package frontend

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"reflect"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/opiproject/gospdk/spdk"
	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/server"
)

type frontendClient struct {
	pb.FrontendNvmeServiceClient
}

type testEnv struct {
	opiSpdkServer *Server
	client        *frontendClient
	ln            net.Listener
	testSocket    string
	ctx           context.Context
	conn          *grpc.ClientConn
	jsonRPC       spdk.JSONRPC
}

func (e *testEnv) Close() {
	server.CloseListener(e.ln)
	server.CloseGrpcConnection(e.conn)
	if err := os.RemoveAll(e.testSocket); err != nil {
		log.Fatal(err)
	}
}

func createTestEnvironment(startSpdkServer bool, spdkResponses []string) *testEnv {
	env := &testEnv{}
	env.testSocket = server.GenerateSocketName("frontend")
	env.ln, env.jsonRPC = server.CreateTestSpdkServer(env.testSocket, startSpdkServer, spdkResponses)
	env.opiSpdkServer = NewServer(env.jsonRPC)

	ctx := context.Background()
	conn, err := grpc.DialContext(ctx,
		"",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(dialer(env.opiSpdkServer)))
	if err != nil {
		log.Fatal(err)
	}
	env.ctx = ctx
	env.conn = conn

	env.client = &frontendClient{
		pb.NewFrontendNvmeServiceClient(env.conn),
	}

	return env
}

func dialer(opiSpdkServer *Server) func(context.Context, string) (net.Conn, error) {
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	pb.RegisterFrontendNvmeServiceServer(server, opiSpdkServer)

	go func() {
		if err := server.Serve(listener); err != nil {
			log.Fatal(err)
		}
	}()

	return func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}
}

var (
	testSubsystemID = "subsystem-test"
	testSubsystem   = pb.NVMeSubsystem{
		Spec: &pb.NVMeSubsystemSpec{
			Nqn: "nqn.2022-09.io.spdk:opi3",
		},
	}
	testControllerID = "controller-test"
	testController   = pb.NVMeController{
		Spec: &pb.NVMeControllerSpec{
			SubsystemId:      &pc.ObjectKey{Value: testSubsystemID},
			PcieId:           &pb.PciEndpoint{PhysicalFunction: 1, VirtualFunction: 2},
			NvmeControllerId: 17,
		},
		Status: &pb.NVMeControllerStatus{
			Active: true,
		},
	}
	testNamespaceID = "namespace-test"
	testNamespace   = pb.NVMeNamespace{
		Spec: &pb.NVMeNamespaceSpec{
			HostNsid:    22,
			SubsystemId: &pc.ObjectKey{Value: testSubsystemID},
		},
		Status: &pb.NVMeNamespaceStatus{
			PciState:     2,
			PciOperState: 1,
		},
	}
)

func TestFrontEnd_CreateNVMeSubsystem(t *testing.T) {
	spec := &pb.NVMeSubsystemSpec{
		Name:         testSubsystemID,
		Nqn:          "nqn.2022-09.io.spdk:opi3",
		SerialNumber: "OpiSerialNumber",
		ModelNumber:  "OpiModelNumber",
	}
	tests := map[string]struct {
		in      *pb.NVMeSubsystem
		out     *pb.NVMeSubsystem
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
		exist   bool
	}{
		"valid request with invalid SPDK response": {
			&pb.NVMeSubsystem{
				Spec: spec,
			},
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result": {"status": 1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not create NQN: %v", "nqn.2022-09.io.spdk:opi3"),
			true,
			false,
		},
		"valid request with empty SPDK response": {
			&pb.NVMeSubsystem{
				Spec: spec,
			},
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_create_subsystem: %v", "EOF"),
			true,
			false,
		},
		"valid request with ID mismatch SPDK response": {
			&pb.NVMeSubsystem{
				Spec: spec,
			},
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_create_subsystem: %v", "json response ID mismatch"),
			true,
			false,
		},
		"valid request with error code from SPDK response": {
			&pb.NVMeSubsystem{
				Spec: spec,
			},
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_create_subsystem: %v", "json response error: myopierr"),
			true,
			false,
		},
		"valid request with error code from SPDK version response": {
			&pb.NVMeSubsystem{
				Spec: spec,
			},
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0}}`, `{"id":%d,"error":{"code":1,"message":"myopierr"},"result":false}`},
			codes.Unknown,
			fmt.Sprintf("spdk_get_version: %v", "json response error: myopierr"),
			true,
			false,
		},
		"valid request with valid SPDK response": {
			&pb.NVMeSubsystem{
				Spec: spec,
			},
			&pb.NVMeSubsystem{
				Spec: spec,
				Status: &pb.NVMeSubsystemStatus{
					FirmwareRevision: "SPDK v20.10",
				},
			},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0}}`, `{"jsonrpc":"2.0","id":%d,"result":{"version":"SPDK v20.10","fields":{"major":20,"minor":10,"patch":0,"suffix":""}}}`},
			codes.OK,
			"",
			true,
			false,
		},
		"already exists": {
			&pb.NVMeSubsystem{
				Spec: spec,
			},
			&testSubsystem,
			[]string{""},
			codes.OK,
			"",
			false,
			true,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Controllers[testControllerID] = &testController
			testEnv.opiSpdkServer.Namespaces[testNamespaceID] = &testNamespace
			if tt.exist {
				testEnv.opiSpdkServer.Subsystems[testSubsystemID] = &testSubsystem
				// testEnv.opiSpdkServer.Subsystems[testSubsystemID].Spec.Id = &pc.ObjectKey{Value: testSubsystemID}
			}
			if tt.out != nil {
				tt.out.Spec.Name = testSubsystemID
			}

			request := &pb.CreateNVMeSubsystemRequest{NvMeSubsystem: tt.in, NvMeSubsystemId: testSubsystemID}
			response, err := testEnv.client.CreateNVMeSubsystem(testEnv.ctx, request)
			if response != nil {
				// Marshall the request and response, so we can just compare the contained data
				mtt, _ := proto.Marshal(tt.out)
				mResponse, _ := proto.Marshal(response)

				// Compare the marshalled messages
				if !bytes.Equal(mtt, mResponse) {
					t.Error("response: expected", tt.out, "received", response)
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
	tests := map[string]struct {
		in      *pb.NVMeSubsystem
		out     *pb.NVMeSubsystem
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		"unimplemented method": {
			&pb.NVMeSubsystem{},
			nil,
			[]string{""},
			codes.Unimplemented,
			fmt.Sprintf("%v method is not implemented", "UpdateNVMeSubsystem"),
			false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Subsystems[testSubsystemID] = &testSubsystem
			testEnv.opiSpdkServer.Controllers[testControllerID] = &testController
			testEnv.opiSpdkServer.Namespaces[testNamespaceID] = &testNamespace

			request := &pb.UpdateNVMeSubsystemRequest{NvMeSubsystem: tt.in}
			response, err := testEnv.client.UpdateNVMeSubsystem(testEnv.ctx, request)
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
	tests := map[string]struct {
		out     []*pb.NVMeSubsystem
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
		size    int32
		token   string
	}{
		"valid request with invalid SPDK response": {
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not list %v", "subsystems"),
			true,
			0,
			"",
		},
		"valid request with empty SPDK response": {
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_get_subsys_list: %v", "EOF"),
			true,
			0,
			"",
		},
		"valid request with ID mismatch SPDK response": {
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_get_subsys_list: %v", "json response ID mismatch"),
			true,
			0,
			"",
		},
		"valid request with error code from SPDK response": {
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_get_subsys_list: %v", "json response error: myopierr"),
			true,
			0,
			"",
		},
		"pagination negative": {
			nil,
			[]string{},
			codes.InvalidArgument,
			"negative PageSize is not allowed",
			false,
			-10,
			"",
		},
		"pagination error": {
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find pagination token %s", "unknown-pagination-token"),
			false,
			0,
			"unknown-pagination-token",
		},
		"pagination": {
			[]*pb.NVMeSubsystem{
				{Spec: &pb.NVMeSubsystemSpec{Nqn: "nqn.2022-09.io.spdk:opi1"}},
			},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0, "subsys_list": [{"subnqn": "nqn.2022-09.io.spdk:opi1"},{"subnqn": "nqn.2022-09.io.spdk:opi2"},{"subnqn": "nqn.2022-09.io.spdk:opi3"}]}}`},
			codes.OK,
			"",
			true,
			1,
			"",
		},
		"pagination overflow": {
			[]*pb.NVMeSubsystem{
				{Spec: &pb.NVMeSubsystemSpec{Nqn: "nqn.2022-09.io.spdk:opi1"}},
				{Spec: &pb.NVMeSubsystemSpec{Nqn: "nqn.2022-09.io.spdk:opi2"}},
				{Spec: &pb.NVMeSubsystemSpec{Nqn: "nqn.2022-09.io.spdk:opi3"}},
			},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0, "subsys_list": [{"subnqn": "nqn.2022-09.io.spdk:opi1"},{"subnqn": "nqn.2022-09.io.spdk:opi2"},{"subnqn": "nqn.2022-09.io.spdk:opi3"}]}}`},
			codes.OK,
			"",
			true,
			1000,
			"",
		},
		"pagination offset": {
			[]*pb.NVMeSubsystem{
				{Spec: &pb.NVMeSubsystemSpec{Nqn: "nqn.2022-09.io.spdk:opi2"}},
			},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0, "subsys_list": [{"subnqn": "nqn.2022-09.io.spdk:opi1"},{"subnqn": "nqn.2022-09.io.spdk:opi2"},{"subnqn": "nqn.2022-09.io.spdk:opi3"}]}}`},
			codes.OK,
			"",
			true,
			1,
			"existing-pagination-token",
		},
		"valid request with valid SPDK response": {
			[]*pb.NVMeSubsystem{
				{Spec: &pb.NVMeSubsystemSpec{Nqn: "nqn.2022-09.io.spdk:opi1"}},
				{Spec: &pb.NVMeSubsystemSpec{Nqn: "nqn.2022-09.io.spdk:opi2"}},
				{Spec: &pb.NVMeSubsystemSpec{Nqn: "nqn.2022-09.io.spdk:opi3"}},
			},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0, "subsys_list": [{"subnqn": "nqn.2022-09.io.spdk:opi1"},{"subnqn": "nqn.2022-09.io.spdk:opi2"},{"subnqn": "nqn.2022-09.io.spdk:opi3"}]}}`},
			codes.OK,
			"",
			true,
			0,
			"",
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Subsystems[testSubsystemID] = &testSubsystem
			testEnv.opiSpdkServer.Controllers[testControllerID] = &testController
			testEnv.opiSpdkServer.Namespaces[testNamespaceID] = &testNamespace
			testEnv.opiSpdkServer.Pagination["existing-pagination-token"] = 1

			request := &pb.ListNVMeSubsystemsRequest{PageSize: tt.size, PageToken: tt.token}
			response, err := testEnv.client.ListNVMeSubsystems(testEnv.ctx, request)
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
	tests := map[string]struct {
		in      string
		out     *pb.NVMeSubsystem
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		"valid request with invalid SPDK response": {
			testSubsystemID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not list NQN: %v", "nqn.2022-09.io.spdk:opi3"),
			true,
		},
		"valid request with empty SPDK response": {
			testSubsystemID,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_get_subsys_list: %v", "EOF"),
			true,
		},
		"valid request with ID mismatch SPDK response": {
			testSubsystemID,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_get_subsys_list: %v", "json response ID mismatch"),
			true,
		},
		"valid request with error code from SPDK response": {
			testSubsystemID,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_get_subsys_list: %v", "json response error: myopierr"),
			true,
		},
		"valid request with valid SPDK response": {
			testSubsystemID,
			&pb.NVMeSubsystem{
				Spec:   &pb.NVMeSubsystemSpec{Nqn: "nqn.2022-09.io.spdk:opi3"},
				Status: &pb.NVMeSubsystemStatus{FirmwareRevision: "TBD"},
			},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0, "subsys_list": [{"subnqn": "nqn.2022-09.io.spdk:opi1"},{"subnqn": "nqn.2022-09.io.spdk:opi2"},{"subnqn": "nqn.2022-09.io.spdk:opi3"}]}}`},
			codes.OK,
			"",
			true,
		},
		"valid request with unknown key": {
			"unknown-subsystem-id",
			nil,
			[]string{""},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", "unknown-subsystem-id"),
			false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Subsystems[testSubsystemID] = &testSubsystem
			testEnv.opiSpdkServer.Controllers[testControllerID] = &testController
			testEnv.opiSpdkServer.Namespaces[testNamespaceID] = &testNamespace

			request := &pb.GetNVMeSubsystemRequest{Name: tt.in}
			response, err := testEnv.client.GetNVMeSubsystem(testEnv.ctx, request)
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
	tests := map[string]struct {
		in      string
		out     *pb.VolumeStats
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		"valid request with invalid SPDK response": {
			testSubsystemID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not stats NQN: %v", "nqn.2022-09.io.spdk:opi3"),
			true,
		},
		"valid request with invalid marshal SPDK response": {
			testSubsystemID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_get_info: %v", "json: cannot unmarshal array into Go value of type models.MrvlNvmGetSubsysInfoResult"),
			true,
		},
		"valid request with empty SPDK response": {
			testSubsystemID,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_get_info: %v", "EOF"),
			true,
		},
		"valid request with ID mismatch SPDK response": {
			testSubsystemID,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_get_info: %v", "json response ID mismatch"),
			true,
		},
		"valid request with error code from SPDK response": {
			testSubsystemID,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_get_info: %v", "json response error: myopierr"),
			true,
		},
		"valid request with valid SPDK response": {
			testSubsystemID,
			&pb.VolumeStats{
				ReadOpsCount:  -1,
				WriteOpsCount: -1,
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":{"status":0,"subsys_list":[{"subnqn":"nqn.2014-08.org.nvmexpress.discovery","mn":"OCTEON NVME 0.0.1","sn":"OCTNVME0000000000002","max_namespaces":16,"min_ctrlr_id":1,"max_ctrlr_id":8,"num_ns":2,"num_total_ctrlr":2,"num_active_ctrlr":2,"ns_list":[{"ns_instance_id":1,"bdev":"bdev01","ctrlr_id_list":[{"ctrlr_id":1},{"ctrlr_id":2}]},{"ns_instance_id":1,"bdev":"bdev02","ctrlr_id_list":[{"ctrlr_id":3}]}]}]}}`},
			codes.OK,
			"",
			true,
		},
		"valid request with unknown key": {
			"unknown-subsystem-id",
			nil,
			[]string{""},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", "unknown-subsystem-id"),
			false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Subsystems[testSubsystemID] = &testSubsystem
			testEnv.opiSpdkServer.Controllers[testControllerID] = &testController
			testEnv.opiSpdkServer.Namespaces[testNamespaceID] = &testNamespace

			request := &pb.NVMeSubsystemStatsRequest{SubsystemId: &pc.ObjectKey{Value: tt.in}}
			response, err := testEnv.client.NVMeSubsystemStats(testEnv.ctx, request)
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
		Name:             testControllerID,
		SubsystemId:      &pc.ObjectKey{Value: testSubsystemID},
		PcieId:           &pb.PciEndpoint{PhysicalFunction: 1, VirtualFunction: 2, PortId: 3},
		NvmeControllerId: 1,
		MaxNsq:           5,
		MaxNcq:           6,
		Sqes:             7,
		Cqes:             8,
	}
	tests := map[string]struct {
		in      *pb.NVMeController
		out     *pb.NVMeController
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
		exist   bool
	}{
		"valid request with invalid SPDK response": {
			&pb.NVMeController{
				Spec: spec,
			},
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1, "cntlid": -1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not create CTRL: %v", testControllerID),
			true,
			false,
		},
		"valid request with empty SPDK response": {
			&pb.NVMeController{
				Spec: spec,
			},
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_create_ctrlr: %v", "EOF"),
			true,
			false,
		},
		"valid request with ID mismatch SPDK response": {
			&pb.NVMeController{
				Spec: spec,
			},
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1, "cntlid": 17}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_create_ctrlr: %v", "json response ID mismatch"),
			true,
			false,
		},
		"valid request with error code from SPDK response": {
			&pb.NVMeController{
				Spec: spec,
			},
			nil,
			[]string{`{"id":%d,"error":{"code":-32602,"message":"Invalid parameters"}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_create_ctrlr: %v", "json response error: Invalid parameters"),
			true,
			false,
		},
		"valid request with valid SPDK response": {
			&pb.NVMeController{
				Spec: &pb.NVMeControllerSpec{
					Name:             testControllerID,
					SubsystemId:      &pc.ObjectKey{Value: testSubsystemID},
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
					Name:        testControllerID,
					SubsystemId: &pc.ObjectKey{Value: testSubsystemID},
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
			false,
		},
		"already exists": {
			&pb.NVMeController{
				Spec: spec,
			},
			&testController,
			[]string{""},
			codes.OK,
			"",
			false,
			true,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Subsystems[testSubsystemID] = &testSubsystem
			testEnv.opiSpdkServer.Namespaces[testNamespaceID] = &testNamespace
			if tt.exist {
				testEnv.opiSpdkServer.Controllers[testControllerID] = &testController
				// testEnv.opiSpdkServer.Controllers[testControllerID].Spec.Id = &pc.ObjectKey{Value: testControllerID}
			}
			if tt.out != nil {
				tt.out.Spec.Name = testControllerID
			}

			request := &pb.CreateNVMeControllerRequest{NvMeController: tt.in, NvMeControllerId: testControllerID}
			response, err := testEnv.client.CreateNVMeController(testEnv.ctx, request)
			if response != nil {
				// Marshall the request and response, so we can just compare the contained data
				mtt, _ := proto.Marshal(tt.out)
				mResponse, _ := proto.Marshal(response)

				// Compare the marshalled messages
				if !bytes.Equal(mtt, mResponse) {
					t.Error("response: expected", tt.out, "received", response)
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
		Name:             testControllerID,
		SubsystemId:      &pc.ObjectKey{Value: testSubsystemID},
		PcieId:           &pb.PciEndpoint{PhysicalFunction: 1, VirtualFunction: 2, PortId: 3},
		NvmeControllerId: 1,
		MaxNsq:           5,
		MaxNcq:           6,
		Sqes:             7,
		Cqes:             8,
	}
	tests := map[string]struct {
		in      *pb.NVMeController
		out     *pb.NVMeController
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		"valid request with invalid SPDK response": {
			&pb.NVMeController{
				Spec: spec,
			},
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1, "cntlid": -1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not update CTRL: %v", testControllerID),
			true,
		},
		"valid request with empty SPDK response": {
			&pb.NVMeController{
				Spec: spec,
			},
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_update_ctrlr: %v", "EOF"),
			true,
		},
		"valid request with ID mismatch SPDK response": {
			&pb.NVMeController{
				Spec: spec,
			},
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1, "cntlid": 17}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_update_ctrlr: %v", "json response ID mismatch"),
			true,
		},
		"valid request with error code from SPDK response": {
			&pb.NVMeController{
				Spec: spec,
			},
			nil,
			[]string{`{"id":%d,"error":{"code":-32602,"message":"Invalid parameters"}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_update_ctrlr: %v", "json response error: Invalid parameters"),
			true,
		},
		"valid request with valid SPDK response": {
			&pb.NVMeController{
				Spec: &pb.NVMeControllerSpec{
					Name:             testControllerID,
					SubsystemId:      &pc.ObjectKey{Value: testSubsystemID},
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
					Name:        testControllerID,
					SubsystemId: &pc.ObjectKey{Value: testSubsystemID},
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

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Subsystems[testSubsystemID] = &testSubsystem
			testEnv.opiSpdkServer.Controllers[testControllerID] = &testController
			testEnv.opiSpdkServer.Namespaces[testNamespaceID] = &testNamespace

			request := &pb.UpdateNVMeControllerRequest{NvMeController: tt.in}
			response, err := testEnv.client.UpdateNVMeController(testEnv.ctx, request)
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
	tests := map[string]struct {
		in      string
		out     []*pb.NVMeController
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
		size    int32
		token   string
	}{
		"valid request with invalid SPDK response": {
			testSubsystemID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1, "ctrlr_id_list": []}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not list CTRLs: %v", testSubsystemID),
			true,
			0,
			"",
		},
		"valid request with empty SPDK response": {
			testSubsystemID,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_get_ctrlr_list: %v", "EOF"),
			true,
			0,
			"",
		},
		"valid request with ID mismatch SPDK response": {
			testSubsystemID,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1, "ctrlr_id_list": []}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_get_ctrlr_list: %v", "json response ID mismatch"),
			true,
			0,
			"",
		},
		"valid request with error code from SPDK response": {
			testSubsystemID,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":{"status": 1, "ctrlr_id_list": []}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_get_ctrlr_list: %v", "json response error: myopierr"),
			true,
			0,
			"",
		},
		"pagination negative": {
			testSubsystemID,
			nil,
			[]string{},
			codes.InvalidArgument,
			"negative PageSize is not allowed",
			false,
			-10,
			"",
		},
		"pagination error": {
			testSubsystemID,
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find pagination token %s", "unknown-pagination-token"),
			false,
			0,
			"unknown-pagination-token",
		},
		"pagination": {
			testSubsystemID,
			[]*pb.NVMeController{
				{
					Spec: &pb.NVMeControllerSpec{
						NvmeControllerId: 1,
					},
				},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"error":{"code":0,"message":""},"result":{"status":0,"ctrlr_id_list":[{"ctrlr_id":1},{"ctrlr_id":2},{"ctrlr_id":3}]}}`},
			codes.OK,
			"",
			true,
			1,
			"",
		},
		"pagination overflow": {
			testSubsystemID,
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
			1000,
			"",
		},
		"valid request with valid SPDK response": {
			testSubsystemID,
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
			0,
			"",
		},
		"valid request with unknown key": {
			"unknown-subsystem-id",
			nil,
			[]string{""},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", "unknown-subsystem-id"),
			false,
			0,
			"",
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Subsystems[testSubsystemID] = &testSubsystem
			testEnv.opiSpdkServer.Controllers[testControllerID] = &testController
			testEnv.opiSpdkServer.Namespaces[testNamespaceID] = &testNamespace
			testEnv.opiSpdkServer.Pagination["existing-pagination-token"] = 1

			request := &pb.ListNVMeControllersRequest{Parent: tt.in, PageSize: tt.size, PageToken: tt.token}
			response, err := testEnv.client.ListNVMeControllers(testEnv.ctx, request)
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
	tests := map[string]struct {
		in      string
		out     *pb.NVMeController
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		"valid request with invalid SPDK response": {
			testControllerID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status":1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not get CTRL: %v", testControllerID),
			true,
		},
		"valid request with empty SPDK response": {
			testControllerID,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ctrlr_get_info: %v", "EOF"),
			true,
		},
		"valid request with ID mismatch SPDK response": {
			testControllerID,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status":1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ctrlr_get_info: %v", "json response ID mismatch"),
			true,
		},
		"valid request with error code from SPDK response": {
			testControllerID,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":{"status":1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ctrlr_get_info: %v", "json response error: myopierr"),
			true,
		},
		"valid request with valid SPDK response": {
			testControllerID,
			&pb.NVMeController{
				Spec: &pb.NVMeControllerSpec{
					Name:             testControllerID,
					NvmeControllerId: 17,
				},
				Status: &pb.NVMeControllerStatus{Active: true},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":{"status":0,"pcie_domain_id":1,"pf_id":1,"vf_id":1,"ctrlr_id":1,"max_nsq":4,"max_ncq":4,"mqes":2048,"ieee_oui":"005043","cmic":6,"nn":16,"active_ns_count":4,"active_nsq":2,"active_ncq":2,"mdts":9,"sqes":6,"cqes":4}}`},
			codes.OK,
			"",
			true,
		},
		"valid request with unknown key": {
			"unknown-subsystem-id",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("error finding controller %v", "unknown-subsystem-id"),
			false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Subsystems[testSubsystemID] = &testSubsystem
			testEnv.opiSpdkServer.Controllers[testControllerID] = &testController
			testEnv.opiSpdkServer.Namespaces[testNamespaceID] = &testNamespace

			request := &pb.GetNVMeControllerRequest{Name: tt.in}
			response, err := testEnv.client.GetNVMeController(testEnv.ctx, request)
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
	tests := map[string]struct {
		in      string
		out     *pb.VolumeStats
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		"valid request with invalid SPDK response": {
			testControllerID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not stats CTRL: %v", testControllerID),
			true,
		},
		"valid request with invalid marshal SPDK response": {
			testControllerID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_get_ctrlr_stats: %v", "json: cannot unmarshal array into Go value of type models.MrvlNvmGetCtrlrStatsResult"),
			true,
		},
		"valid request with empty SPDK response": {
			testControllerID,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_get_ctrlr_stats: %v", "EOF"),
			true,
		},
		"valid request with ID mismatch SPDK response": {
			testControllerID,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_get_ctrlr_stats: %v", "json response ID mismatch"),
			true,
		},
		"valid request with error code from SPDK response": {
			testControllerID,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_get_ctrlr_stats: %v", "json response error: myopierr"),
			true,
		},
		"valid request with valid SPDK response": {
			testControllerID,
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
		"valid request with unknown key": {
			"unknown-controller-id",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("error finding controller %v", "unknown-controller-id"),
			false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Subsystems[testSubsystemID] = &testSubsystem
			testEnv.opiSpdkServer.Controllers[testControllerID] = &testController
			testEnv.opiSpdkServer.Namespaces[testNamespaceID] = &testNamespace

			request := &pb.NVMeControllerStatsRequest{Id: &pc.ObjectKey{Value: tt.in}}
			response, err := testEnv.client.NVMeControllerStats(testEnv.ctx, request)
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
		Name:        testNamespaceID,
		SubsystemId: &pc.ObjectKey{Value: testSubsystemID},
		HostNsid:    0,
		VolumeId:    &pc.ObjectKey{Value: "Malloc1"},
		Uuid:        &pc.Uuid{Value: "1b4e28ba-2fa1-11d2-883f-b9a761bde3fb"},
		Nguid:       "1b4e28ba-2fa1-11d2-883f-b9a761bde3fb",
		Eui64:       1967554867335598546,
	}
	tests := map[string]struct {
		in      *pb.NVMeNamespace
		out     *pb.NVMeNamespace
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
		exist   bool
	}{
		"valid request with invalid SPDK response": {
			&pb.NVMeNamespace{
				Spec: spec,
			},
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not create NS: %v", testNamespaceID),
			true,
			false,
		},
		"valid request with empty SPDK response": {
			&pb.NVMeNamespace{
				Spec: spec,
			},
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_alloc_ns: %v", "EOF"),
			true,
			false,
		},
		"valid request with ID mismatch SPDK response": {
			&pb.NVMeNamespace{
				Spec: spec,
			},
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_alloc_ns: %v", "json response ID mismatch"),
			true,
			false,
		},
		"valid request with error code from SPDK response": {
			&pb.NVMeNamespace{
				Spec: spec,
			},
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_alloc_ns: %v", "json response error: myopierr"),
			true,
			false,
		},
		"valid request with valid SPDK response": {
			&pb.NVMeNamespace{
				Spec: &pb.NVMeNamespaceSpec{
					Name:        testNamespaceID,
					SubsystemId: &pc.ObjectKey{Value: testSubsystemID},
					HostNsid:    22,
					VolumeId:    &pc.ObjectKey{Value: "Malloc1"},
					Uuid:        &pc.Uuid{Value: "1b4e28ba-2fa1-11d2-883f-b9a761bde3fb"},
					Nguid:       "1b4e28ba-2fa1-11d2-883f-b9a761bde3fb",
					Eui64:       1967554867335598546,
				},
			},
			&pb.NVMeNamespace{
				Spec: &pb.NVMeNamespaceSpec{
					Name:        testNamespaceID,
					SubsystemId: &pc.ObjectKey{Value: testSubsystemID},
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
			false,
		},
		"valid request with invalid SPDK second attach response": {
			&pb.NVMeNamespace{
				Spec: spec,
			},
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0, "ns_instance_id": 17}}`, `{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not attach NS: %v", testNamespaceID),
			true,
			false,
		},
		"already exists": {
			&pb.NVMeNamespace{
				Spec: spec,
			},
			&testNamespace,
			[]string{""},
			codes.OK,
			"",
			false,
			true,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Subsystems[testSubsystemID] = &testSubsystem
			testEnv.opiSpdkServer.Subsystems[testSubsystemID].Spec.Name = testSubsystemID
			testEnv.opiSpdkServer.Controllers[testControllerID] = &testController
			if tt.exist {
				testEnv.opiSpdkServer.Namespaces[testNamespaceID] = &testNamespace
				testEnv.opiSpdkServer.Namespaces[testNamespaceID].Spec.Name = testNamespaceID
			}
			if tt.out != nil {
				tt.out.Spec.Name = testNamespaceID
			}

			request := &pb.CreateNVMeNamespaceRequest{NvMeNamespace: tt.in, NvMeNamespaceId: testNamespaceID}
			response, err := testEnv.client.CreateNVMeNamespace(testEnv.ctx, request)
			if response != nil {
				// Marshall the request and response, so we can just compare the contained data
				mtt, _ := proto.Marshal(tt.out)
				mResponse, _ := proto.Marshal(response)

				// Compare the marshalled messages
				if !bytes.Equal(mtt, mResponse) {
					t.Error("response: expected", tt.out, "received", response)
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
	tests := map[string]struct {
		in      *pb.NVMeNamespace
		out     *pb.NVMeNamespace
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		"unimplemented method": {
			&pb.NVMeNamespace{},
			nil,
			[]string{""},
			codes.Unimplemented,
			fmt.Sprintf("%v method is not implemented", "UpdateNVMeNamespace"),
			false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Subsystems[testSubsystemID] = &testSubsystem
			testEnv.opiSpdkServer.Controllers[testControllerID] = &testController
			testEnv.opiSpdkServer.Namespaces[testNamespaceID] = &testNamespace
			request := &pb.UpdateNVMeNamespaceRequest{NvMeNamespace: tt.in}
			response, err := testEnv.client.UpdateNVMeNamespace(testEnv.ctx, request)
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
	tests := map[string]struct {
		in      string
		out     []*pb.NVMeNamespace
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
		size    int32
		token   string
	}{
		"valid request with invalid SPDK response": {
			testSubsystemID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status":1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not list NS: %v", testSubsystemID),
			true,
			0,
			"",
		},
		"valid request with invalid marshal SPDK response": {
			testSubsystemID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_get_ns_list: %v", "json: cannot unmarshal array into Go value of type models.MrvlNvmSubsysGetNsListResult"),
			true,
			0,
			"",
		},
		"valid request with empty SPDK response": {
			testSubsystemID,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_get_ns_list: %v", "EOF"),
			true,
			0,
			"",
		},
		"valid request with ID mismatch SPDK response": {
			testSubsystemID,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status":1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_get_ns_list: %v", "json response ID mismatch"),
			true,
			0,
			"",
		},
		"valid request with error code from SPDK response": {
			testSubsystemID,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_get_ns_list: %v", "json response error: myopierr"),
			true,
			0,
			"",
		},
		"pagination negative": {
			testSubsystemID,
			nil,
			[]string{},
			codes.InvalidArgument,
			"negative PageSize is not allowed",
			false,
			-10,
			"",
		},
		"pagination error": {
			testSubsystemID,
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find pagination token %s", "unknown-pagination-token"),
			false,
			0,
			"unknown-pagination-token",
		},
		"pagination": {
			testSubsystemID,
			[]*pb.NVMeNamespace{
				{
					Spec: &pb.NVMeNamespaceSpec{
						HostNsid: 11,
					},
				},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":{"status":0,"ns_list":[{"ns_instance_id":11,"bdev":"bdev01","ctrlr_id_list":[{"ctrlr_id":1}]},{"ns_instance_id":12,"bdev":"bdev02","ctrlr_id_list":[]},{"ns_instance_id":13,"bdev":"bdev03","ctrlr_id_list":[]}]}}`},
			codes.OK,
			"",
			true,
			1,
			"",
		},
		"pagination overflow": {
			testSubsystemID,
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
			1000,
			"",
		},
		"pagination offset": {
			testSubsystemID,
			[]*pb.NVMeNamespace{
				{
					Spec: &pb.NVMeNamespaceSpec{
						HostNsid: 12,
					},
				},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":{"status":0,"ns_list":[{"ns_instance_id":11,"bdev":"bdev01","ctrlr_id_list":[{"ctrlr_id":1}]},{"ns_instance_id":12,"bdev":"bdev02","ctrlr_id_list":[]},{"ns_instance_id":13,"bdev":"bdev03","ctrlr_id_list":[]}]}}`},
			codes.OK,
			"",
			true,
			1,
			"existing-pagination-token",
		},
		"valid request with valid SPDK response": {
			testSubsystemID,
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
			0,
			"",
		},
		"valid request with unknown key": {
			"unknown-namespace-id",
			nil,
			[]string{""},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", "unknown-namespace-id"),
			false,
			0,
			"",
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Subsystems[testSubsystemID] = &testSubsystem
			testEnv.opiSpdkServer.Controllers[testControllerID] = &testController
			testEnv.opiSpdkServer.Namespaces[testNamespaceID] = &testNamespace
			testEnv.opiSpdkServer.Pagination["existing-pagination-token"] = 1

			request := &pb.ListNVMeNamespacesRequest{Parent: tt.in, PageSize: tt.size, PageToken: tt.token}
			response, err := testEnv.client.ListNVMeNamespaces(testEnv.ctx, request)
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
	tests := map[string]struct {
		in      string
		out     *pb.NVMeNamespace
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		"valid request with invalid SPDK response": {
			testNamespaceID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status":1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not get NS: %v", testNamespaceID),
			true,
		},
		"valid request with invalid marshal SPDK response": {
			testNamespaceID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ns_get_info: %v", "json: cannot unmarshal array into Go value of type models.MrvlNvmGetNsInfoResult"),
			true,
		},
		"valid request with empty SPDK response": {
			testNamespaceID,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ns_get_info: %v", "EOF"),
			true,
		},
		"valid request with ID mismatch SPDK response": {
			testNamespaceID,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status":1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ns_get_info: %v", "json response ID mismatch"),
			true,
		},
		"valid request with error code from SPDK response": {
			testNamespaceID,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ns_get_info: %v", "json response error: myopierr"),
			true,
		},
		"valid request with valid SPDK response": {
			testNamespaceID,
			&pb.NVMeNamespace{
				Spec: &pb.NVMeNamespaceSpec{
					Name:  testNamespaceID,
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
		"valid request with unknown key": {
			"unknown-namespace-id",
			nil,
			[]string{""},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", "unknown-namespace-id"),
			false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Subsystems[testSubsystemID] = &testSubsystem
			testEnv.opiSpdkServer.Subsystems[testSubsystemID].Spec.Name = testSubsystemID
			testEnv.opiSpdkServer.Controllers[testControllerID] = &testController
			testEnv.opiSpdkServer.Controllers[testControllerID].Spec.Name = testControllerID
			testEnv.opiSpdkServer.Namespaces[testNamespaceID] = &testNamespace
			testEnv.opiSpdkServer.Namespaces[testNamespaceID].Spec.Name = testNamespaceID

			request := &pb.GetNVMeNamespaceRequest{Name: tt.in}
			response, err := testEnv.client.GetNVMeNamespace(testEnv.ctx, request)
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
	tests := map[string]struct {
		in      string
		out     *pb.VolumeStats
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		"valid request with invalid SPDK response": {
			testNamespaceID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not stats NS: %v", testNamespaceID),
			true,
		},
		"valid request with invalid marshal SPDK response": {
			testNamespaceID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_get_ns_stats: %v", "json: cannot unmarshal array into Go value of type models.MrvlNvmGetNsStatsResult"),
			true,
		},
		"valid request with empty SPDK response": {
			testNamespaceID,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_get_ns_stats: %v", "EOF"),
			true,
		},
		"valid request with ID mismatch SPDK response": {
			testNamespaceID,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_get_ns_stats: %v", "json response ID mismatch"),
			true,
		},
		"valid request with error code from SPDK response": {
			testNamespaceID,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_get_ns_stats: %v", "json response error: myopierr"),
			true,
		},
		"valid request with valid SPDK response": {
			testNamespaceID,
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
		"valid request with unknown key": {
			"unknown-namespace-id",
			nil,
			[]string{""},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", "unknown-namespace-id"),
			false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Subsystems[testSubsystemID] = &testSubsystem
			testEnv.opiSpdkServer.Controllers[testControllerID] = &testController
			testEnv.opiSpdkServer.Namespaces[testNamespaceID] = &testNamespace

			request := &pb.NVMeNamespaceStatsRequest{NamespaceId: &pc.ObjectKey{Value: tt.in}}
			response, err := testEnv.client.NVMeNamespaceStats(testEnv.ctx, request)
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
	tests := map[string]struct {
		in      string
		out     *emptypb.Empty
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
		missing bool
	}{
		"valid request with invalid SPDK response": {
			testNamespaceID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not detach NS: %v", testNamespaceID),
			true,
			false,
		},
		"valid request with invalid SPDK second response": {
			testNamespaceID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0}}`, `{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not delete NS: %v", testNamespaceID),
			true,
			false,
		},
		"valid request with empty SPDK response": {
			testNamespaceID,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ctrlr_detach_ns: %v", "EOF"),
			true,
			false,
		},
		"valid request with ID mismatch SPDK response": {
			testNamespaceID,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ctrlr_detach_ns: %v", "json response ID mismatch"),
			true,
			false,
		},
		"valid request with error code from SPDK response": {
			testNamespaceID,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ctrlr_detach_ns: %v", "json response error: myopierr"),
			true,
			false,
		},
		"valid request with valid SPDK response": {
			testNamespaceID,
			&emptypb.Empty{},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0}}`, `{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0}}`},
			codes.OK,
			"",
			true,
			false,
		},
		"valid request with unknown key": {
			"unknown-namespace-id",
			nil,
			[]string{""},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", "unknown-namespace-id"),
			false,
			false,
		},
		"unknown key with missing allowed": {
			"unknown-id",
			&emptypb.Empty{},
			[]string{""},
			codes.OK,
			"",
			false,
			true,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Subsystems[testSubsystemID] = &testSubsystem
			testEnv.opiSpdkServer.Controllers[testControllerID] = &testController
			testEnv.opiSpdkServer.Namespaces[testNamespaceID] = &testNamespace
			request := &pb.DeleteNVMeNamespaceRequest{Name: tt.in, AllowMissing: tt.missing}
			response, err := testEnv.client.DeleteNVMeNamespace(testEnv.ctx, request)
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
	tests := map[string]struct {
		in      string
		out     *emptypb.Empty
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
		missing bool
	}{
		"valid request with invalid SPDK response": {
			testControllerID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not delete CTRL: %v", testControllerID),
			true,
			false,
		},
		"valid request with empty SPDK response": {
			testControllerID,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_remove_ctrlr: %v", "EOF"),
			true,
			false,
		},
		"valid request with ID mismatch SPDK response": {
			testControllerID,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_remove_ctrlr: %v", "json response ID mismatch"),
			true,
			false,
		},
		"valid request with error code from SPDK response": {
			testControllerID,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_remove_ctrlr: %v", "json response error: myopierr"),
			true,
			false,
		},
		"valid request with valid SPDK response": {
			testControllerID,
			&emptypb.Empty{},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0}}`},
			codes.OK,
			"",
			true,
			false,
		},
		"valid request with unknown key": {
			"unknown-controller-id",
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("error finding controller %v", "unknown-controller-id"),
			false,
			false,
		},
		"unknown key with missing allowed": {
			"unknown-id",
			&emptypb.Empty{},
			[]string{""},
			codes.OK,
			"",
			false,
			true,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Subsystems[testSubsystemID] = &testSubsystem
			testEnv.opiSpdkServer.Controllers[testControllerID] = &testController
			testEnv.opiSpdkServer.Namespaces[testNamespaceID] = &testNamespace

			request := &pb.DeleteNVMeControllerRequest{Name: tt.in, AllowMissing: tt.missing}
			response, err := testEnv.client.DeleteNVMeController(testEnv.ctx, request)
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
	tests := map[string]struct {
		in      string
		out     *emptypb.Empty
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
		missing bool
	}{
		"valid request with invalid SPDK response": {
			testSubsystemID,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not delete NQN: %v", "nqn.2022-09.io.spdk:opi3"),
			true,
			false,
		},
		"valid request with empty SPDK response": {
			testSubsystemID,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_delete_subsystem: %v", "EOF"),
			true,
			false,
		},
		"valid request with ID mismatch SPDK response": {
			testSubsystemID,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_delete_subsystem: %v", "json response ID mismatch"),
			true,
			false,
		},
		"valid request with error code from SPDK response": {
			testSubsystemID,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_delete_subsystem: %v", "json response error: myopierr"),
			true,
			false,
		},
		"valid request with valid SPDK response": {
			testSubsystemID,
			&emptypb.Empty{},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0}}`},
			codes.OK,
			"",
			true,
			false,
		},
		"valid request with unknown key": {
			"unknown-subsystem-id",
			nil,
			[]string{""},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", "unknown-subsystem-id"),
			false,
			false,
		},
		"unknown key with missing allowed": {
			"unknown-id",
			&emptypb.Empty{},
			[]string{""},
			codes.OK,
			"",
			false,
			true,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Subsystems[testSubsystemID] = &testSubsystem
			testEnv.opiSpdkServer.Controllers[testControllerID] = &testController
			testEnv.opiSpdkServer.Namespaces[testNamespaceID] = &testNamespace

			request := &pb.DeleteNVMeSubsystemRequest{Name: tt.in, AllowMissing: tt.missing}
			response, err := testEnv.client.DeleteNVMeSubsystem(testEnv.ctx, request)
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
