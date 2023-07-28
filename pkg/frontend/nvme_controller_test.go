// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2022 Marvell International Ltd.
// Copyright (C) 2023 Intel Corporation

// Package frontend implememnts the FrontEnd APIs (host facing) of the storage Server
package frontend

import (
	"fmt"
	"reflect"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/server"
)

func TestFrontEnd_CreateNvmeController(t *testing.T) {
	spec := &pb.NvmeControllerSpec{
		SubsystemNameRef: testSubsystemName,
		PcieId:           &pb.PciEndpoint{PhysicalFunction: 1, VirtualFunction: 2, PortId: 3},
		NvmeControllerId: 1,
		MaxNsq:           5,
		MaxNcq:           6,
		Sqes:             7,
		Cqes:             8,
	}
	tests := map[string]struct {
		id      string
		in      *pb.NvmeController
		out     *pb.NvmeController
		spdk    []string
		errCode codes.Code
		errMsg  string
		exist   bool
	}{
		"illegal resource_id": {
			"CapitalLettersNotAllowed",
			&pb.NvmeController{
				Spec: spec,
			},
			nil,
			[]string{},
			codes.Unknown,
			fmt.Sprintf("user-settable ID must only contain lowercase, numbers and hyphens (%v)", "got: 'C' in position 0"),
			false,
		},
		"valid request with invalid SPDK response": {
			testControllerID,
			&pb.NvmeController{
				Name: testControllerName,
				Spec: spec,
			},
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1, "cntlid": -1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not create CTRL: %v", testControllerName),
			false,
		},
		"valid request with empty SPDK response": {
			testControllerID,
			&pb.NvmeController{
				Name: testControllerName,
				Spec: spec,
			},
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_create_ctrlr: %v", "EOF"),
			false,
		},
		"valid request with ID mismatch SPDK response": {
			testControllerID,
			&pb.NvmeController{
				Name: testControllerName,
				Spec: spec,
			},
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1, "cntlid": 17}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_create_ctrlr: %v", "json response ID mismatch"),
			false,
		},
		"valid request with error code from SPDK response": {
			testControllerID,
			&pb.NvmeController{
				Name: testControllerName,
				Spec: spec,
			},
			nil,
			[]string{`{"id":%d,"error":{"code":-32602,"message":"Invalid parameters"}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_create_ctrlr: %v", "json response error: Invalid parameters"),
			false,
		},
		"valid request with valid SPDK response": {
			testControllerID,
			&pb.NvmeController{
				Name: testControllerName,
				Spec: &pb.NvmeControllerSpec{
					SubsystemNameRef: testSubsystemName,
					PcieId:           &pb.PciEndpoint{PhysicalFunction: 1, VirtualFunction: 2, PortId: 3},
					NvmeControllerId: 17,
					MaxNsq:           5,
					MaxNcq:           6,
					Sqes:             7,
					Cqes:             8,
				},
			},
			&pb.NvmeController{
				Name: testControllerName,
				Spec: &pb.NvmeControllerSpec{
					SubsystemNameRef: testSubsystemName,
					PcieId:           &pb.PciEndpoint{PhysicalFunction: 1, VirtualFunction: 2, PortId: 3},
					MaxNsq:           5,
					MaxNcq:           6,
					Sqes:             7,
					Cqes:             8,
				},
				Status: &pb.NvmeControllerStatus{Active: true},
			},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0, "cntlid": 17}}`},
			codes.OK,
			"",
			false,
		},
		"already exists": {
			testControllerID,
			&pb.NvmeController{
				Name: testControllerName,
				Spec: spec,
			},
			&testController,
			[]string{},
			codes.OK,
			"",
			true,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Subsystems[testSubsystemName] = &testSubsystem
			testEnv.opiSpdkServer.Namespaces[testNamespaceName] = &testNamespace
			if tt.exist {
				testEnv.opiSpdkServer.Controllers[testControllerName] = &testController
				// testEnv.opiSpdkServer.Controllers[testControllerID].Spec.Id = &pc.ObjectKey{Value: testControllerID}
			}
			if tt.out != nil {
				tt.out.Name = testControllerName
			}

			request := &pb.CreateNvmeControllerRequest{NvmeController: tt.in, NvmeControllerId: tt.id}
			response, err := testEnv.client.CreateNvmeController(testEnv.ctx, request)

			if !proto.Equal(response, tt.out) {
				t.Error("response: expected", tt.out, "received", response)
			}

			if er, ok := status.FromError(err); ok {
				if er.Code() != tt.errCode {
					t.Error("error code: expected", tt.errCode, "received", er.Code())
				}
				if er.Message() != tt.errMsg {
					t.Error("error message: expected", tt.errMsg, "received", er.Message())
				}
			} else {
				t.Error("expected grpc error status")
			}
		})
	}
}

func TestFrontEnd_DeleteNvmeController(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *emptypb.Empty
		spdk    []string
		errCode codes.Code
		errMsg  string
		missing bool
	}{
		"valid request with invalid SPDK response": {
			testControllerName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not delete CTRL: %v", testControllerName),
			false,
		},
		"valid request with empty SPDK response": {
			testControllerName,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_remove_ctrlr: %v", "EOF"),
			false,
		},
		"valid request with ID mismatch SPDK response": {
			testControllerName,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_remove_ctrlr: %v", "json response ID mismatch"),
			false,
		},
		"valid request with error code from SPDK response": {
			testControllerName,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_remove_ctrlr: %v", "json response error: myopierr"),
			false,
		},
		"valid request with valid SPDK response": {
			testControllerName,
			&emptypb.Empty{},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0}}`},
			codes.OK,
			"",
			false,
		},
		"valid request with unknown key": {
			server.ResourceIDToVolumeName("unknown-controller-id"),
			nil,
			[]string{},
			codes.Unknown,
			fmt.Sprintf("error finding controller %v", server.ResourceIDToVolumeName("unknown-controller-id")),
			false,
		},
		"unknown key with missing allowed": {
			"unknown-id",
			&emptypb.Empty{},
			[]string{},
			codes.OK,
			"",
			true,
		},
		"malformed name": {
			"-ABC-DEF",
			&emptypb.Empty{},
			[]string{},
			codes.Unknown,
			fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
			false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Subsystems[testSubsystemName] = &testSubsystem
			testEnv.opiSpdkServer.Controllers[testControllerName] = &testController
			testEnv.opiSpdkServer.Namespaces[testNamespaceName] = &testNamespace

			request := &pb.DeleteNvmeControllerRequest{Name: tt.in, AllowMissing: tt.missing}
			response, err := testEnv.client.DeleteNvmeController(testEnv.ctx, request)

			if er, ok := status.FromError(err); ok {
				if er.Code() != tt.errCode {
					t.Error("error code: expected", tt.errCode, "received", er.Code())
				}
				if er.Message() != tt.errMsg {
					t.Error("error message: expected", tt.errMsg, "received", er.Message())
				}
			} else {
				t.Error("expected grpc error status")
			}

			if reflect.TypeOf(response) != reflect.TypeOf(tt.out) {
				t.Error("response: expected", reflect.TypeOf(tt.out), "received", reflect.TypeOf(response))
			}
		})
	}
}

func TestFrontEnd_UpdateNvmeController(t *testing.T) {
	spec := &pb.NvmeControllerSpec{
		SubsystemNameRef: testSubsystemName,
		PcieId:           &pb.PciEndpoint{PhysicalFunction: 1, VirtualFunction: 2, PortId: 3},
		NvmeControllerId: 1,
		MaxNsq:           5,
		MaxNcq:           6,
		Sqes:             7,
		Cqes:             8,
	}
	tests := map[string]struct {
		mask    *fieldmaskpb.FieldMask
		in      *pb.NvmeController
		out     *pb.NvmeController
		spdk    []string
		errCode codes.Code
		errMsg  string
	}{
		"invalid fieldmask": {
			&fieldmaskpb.FieldMask{Paths: []string{"*", "author"}},
			&pb.NvmeController{
				Name: testControllerName,
				Spec: spec,
			},
			nil,
			[]string{},
			codes.Unknown,
			fmt.Sprintf("invalid field path: %s", "'*' must not be used with other paths"),
		},
		"valid request with invalid SPDK response": {
			nil,
			&pb.NvmeController{
				Name: testControllerName,
				Spec: spec,
			},
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1, "cntlid": -1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not update CTRL: %v", testControllerName),
		},
		"valid request with empty SPDK response": {
			nil,
			&pb.NvmeController{
				Name: testControllerName,
				Spec: spec,
			},
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_update_ctrlr: %v", "EOF"),
		},
		"valid request with ID mismatch SPDK response": {
			nil,
			&pb.NvmeController{
				Name: testControllerName,
				Spec: spec,
			},
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1, "cntlid": 17}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_update_ctrlr: %v", "json response ID mismatch"),
		},
		"valid request with error code from SPDK response": {
			nil,
			&pb.NvmeController{
				Name: testControllerName,
				Spec: spec,
			},
			nil,
			[]string{`{"id":%d,"error":{"code":-32602,"message":"Invalid parameters"}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_update_ctrlr: %v", "json response error: Invalid parameters"),
		},
		"valid request with valid SPDK response": {
			nil,
			&pb.NvmeController{
				Name: testControllerName,
				Spec: &pb.NvmeControllerSpec{
					SubsystemNameRef: testSubsystemName,
					PcieId:           &pb.PciEndpoint{PhysicalFunction: 1, VirtualFunction: 2, PortId: 3},
					NvmeControllerId: 17,
					MaxNsq:           5,
					MaxNcq:           6,
					Sqes:             7,
					Cqes:             8,
				},
			},
			&pb.NvmeController{
				Name: testControllerName,
				Spec: &pb.NvmeControllerSpec{
					SubsystemNameRef: testSubsystemName,
					PcieId:           &pb.PciEndpoint{PhysicalFunction: 1, VirtualFunction: 2, PortId: 3},
					MaxNsq:           5,
					MaxNcq:           6,
					Sqes:             7,
					Cqes:             8,
				},
				Status: &pb.NvmeControllerStatus{Active: true},
			},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0, "cntlid": 17}}`},
			codes.OK,
			"",
		},
		"valid request with unknown key": {
			nil,
			&pb.NvmeController{
				Name: server.ResourceIDToVolumeName("unknown-id"),
			},
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", server.ResourceIDToVolumeName("unknown-id")),
		},
		"malformed name": {
			nil,
			&pb.NvmeController{Name: "-ABC-DEF"},
			nil,
			[]string{},
			codes.Unknown,
			fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Subsystems[testSubsystemName] = &testSubsystem
			testEnv.opiSpdkServer.Controllers[testControllerName] = &testController
			testEnv.opiSpdkServer.Namespaces[testNamespaceName] = &testNamespace

			request := &pb.UpdateNvmeControllerRequest{NvmeController: tt.in, UpdateMask: tt.mask}
			response, err := testEnv.client.UpdateNvmeController(testEnv.ctx, request)

			if !proto.Equal(response, tt.out) {
				t.Error("response: expected", tt.out, "received", response)
			}

			if er, ok := status.FromError(err); ok {
				if er.Code() != tt.errCode {
					t.Error("error code: expected", tt.errCode, "received", er.Code())
				}
				if er.Message() != tt.errMsg {
					t.Error("error message: expected", tt.errMsg, "received", er.Message())
				}
			} else {
				t.Error("expected grpc error status")
			}
		})
	}
}

func TestFrontEnd_ListNvmeControllers(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     []*pb.NvmeController
		spdk    []string
		errCode codes.Code
		errMsg  string
		size    int32
		token   string
	}{
		"valid request with invalid SPDK response": {
			testSubsystemName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1, "ctrlr_id_list": []}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not list CTRLs: %v", testSubsystemName),
			0,
			"",
		},
		"valid request with empty SPDK response": {
			testSubsystemName,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_get_ctrlr_list: %v", "EOF"),
			0,
			"",
		},
		"valid request with ID mismatch SPDK response": {
			testSubsystemName,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1, "ctrlr_id_list": []}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_get_ctrlr_list: %v", "json response ID mismatch"),
			0,
			"",
		},
		"valid request with error code from SPDK response": {
			testSubsystemName,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":{"status": 1, "ctrlr_id_list": []}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_get_ctrlr_list: %v", "json response error: myopierr"),
			0,
			"",
		},
		"pagination negative": {
			testSubsystemName,
			nil,
			[]string{},
			codes.InvalidArgument,
			"negative PageSize is not allowed",
			-10,
			"",
		},
		"pagination error": {
			testSubsystemName,
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find pagination token %s", "unknown-pagination-token"),
			0,
			"unknown-pagination-token",
		},
		"pagination": {
			testSubsystemName,
			[]*pb.NvmeController{
				{
					Spec: &pb.NvmeControllerSpec{
						NvmeControllerId: 1,
					},
				},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"error":{"code":0,"message":""},"result":{"status":0,"ctrlr_id_list":[{"ctrlr_id":1},{"ctrlr_id":2},{"ctrlr_id":3}]}}`},
			codes.OK,
			"",
			1,
			"",
		},
		"pagination overflow": {
			testSubsystemName,
			[]*pb.NvmeController{
				{
					Spec: &pb.NvmeControllerSpec{
						NvmeControllerId: 1,
					},
				},
				{
					Spec: &pb.NvmeControllerSpec{
						NvmeControllerId: 2,
					},
				},
				{
					Spec: &pb.NvmeControllerSpec{
						NvmeControllerId: 3,
					},
				},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"error":{"code":0,"message":""},"result":{"status":0,"ctrlr_id_list":[{"ctrlr_id":1},{"ctrlr_id":2},{"ctrlr_id":3}]}}`},
			codes.OK,
			"",
			1000,
			"",
		},
		"valid request with valid SPDK response": {
			testSubsystemName,
			[]*pb.NvmeController{
				{
					Spec: &pb.NvmeControllerSpec{
						NvmeControllerId: 1,
					},
				},
				{
					Spec: &pb.NvmeControllerSpec{
						NvmeControllerId: 2,
					},
				},
				{
					Spec: &pb.NvmeControllerSpec{
						NvmeControllerId: 3,
					},
				},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"error":{"code":0,"message":""},"result":{"status":0,"ctrlr_id_list":[{"ctrlr_id":1},{"ctrlr_id":2},{"ctrlr_id":3}]}}`},
			codes.OK,
			"",
			0,
			"",
		},
		"valid request with unknown key": {
			"unknown-subsystem-id",
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", "unknown-subsystem-id"),
			0,
			"",
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Subsystems[testSubsystemName] = &testSubsystem
			testEnv.opiSpdkServer.Controllers[testControllerName] = &testController
			testEnv.opiSpdkServer.Namespaces[testNamespaceName] = &testNamespace
			testEnv.opiSpdkServer.Pagination["existing-pagination-token"] = 1

			request := &pb.ListNvmeControllersRequest{Parent: tt.in, PageSize: tt.size, PageToken: tt.token}
			response, err := testEnv.client.ListNvmeControllers(testEnv.ctx, request)

			if !server.EqualProtoSlices(response.GetNvmeControllers(), tt.out) {
				t.Error("response: expected", tt.out, "received", response.GetNvmeControllers())
			}

			// Empty NextPageToken indicates end of results list
			if tt.size != 1 && response.GetNextPageToken() != "" {
				t.Error("Expected end of results, receieved non-empty next page token", response.GetNextPageToken())
			}

			if er, ok := status.FromError(err); ok {
				if er.Code() != tt.errCode {
					t.Error("error code: expected", tt.errCode, "received", er.Code())
				}
				if er.Message() != tt.errMsg {
					t.Error("error message: expected", tt.errMsg, "received", er.Message())
				}
			} else {
				t.Error("expected grpc error status")
			}
		})
	}
}

func TestFrontEnd_GetNvmeController(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *pb.NvmeController
		spdk    []string
		errCode codes.Code
		errMsg  string
	}{
		"valid request with invalid SPDK response": {
			testControllerName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status":1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not get CTRL: %v", testControllerName),
		},
		"valid request with empty SPDK response": {
			testControllerName,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ctrlr_get_info: %v", "EOF"),
		},
		"valid request with ID mismatch SPDK response": {
			testControllerName,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status":1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ctrlr_get_info: %v", "json response ID mismatch"),
		},
		"valid request with error code from SPDK response": {
			testControllerName,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":{"status":1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ctrlr_get_info: %v", "json response error: myopierr"),
		},
		"valid request with valid SPDK response": {
			testControllerName,
			&pb.NvmeController{
				Name: testControllerName,
				Spec: &pb.NvmeControllerSpec{
					NvmeControllerId: 17,
				},
				Status: &pb.NvmeControllerStatus{Active: true},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":{"status":0,"pcie_domain_id":1,"pf_id":1,"vf_id":1,"ctrlr_id":1,"max_nsq":4,"max_ncq":4,"mqes":2048,"ieee_oui":"005043","cmic":6,"nn":16,"active_ns_count":4,"active_nsq":2,"active_ncq":2,"mdts":9,"sqes":6,"cqes":4}}`},
			codes.OK,
			"",
		},
		"valid request with unknown key": {
			server.ResourceIDToVolumeName("unknown-subsystem-id"),
			nil,
			[]string{},
			codes.Unknown,
			fmt.Sprintf("error finding controller %v", server.ResourceIDToVolumeName("unknown-subsystem-id")),
		},
		"malformed name": {
			"-ABC-DEF",
			nil,
			[]string{},
			codes.Unknown,
			fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Subsystems[testSubsystemName] = &testSubsystem
			testEnv.opiSpdkServer.Controllers[testControllerName] = &testController
			testEnv.opiSpdkServer.Namespaces[testNamespaceName] = &testNamespace

			request := &pb.GetNvmeControllerRequest{Name: tt.in}
			response, err := testEnv.client.GetNvmeController(testEnv.ctx, request)

			if !proto.Equal(response, tt.out) {
				t.Error("response: expected", tt.out, "received", response)
			}

			if er, ok := status.FromError(err); ok {
				if er.Code() != tt.errCode {
					t.Error("error code: expected", tt.errCode, "received", er.Code())
				}
				if er.Message() != tt.errMsg {
					t.Error("error message: expected", tt.errMsg, "received", er.Message())
				}
			} else {
				t.Error("expected grpc error status")
			}
		})
	}
}

func TestFrontEnd_NvmeControllerStats(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *pb.VolumeStats
		spdk    []string
		errCode codes.Code
		errMsg  string
	}{
		"valid request with invalid SPDK response": {
			testControllerName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not stats CTRL: %v", testControllerName),
		},
		"valid request with invalid marshal SPDK response": {
			testControllerName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_get_ctrlr_stats: %v", "json: cannot unmarshal array into Go value of type models.MrvlNvmGetCtrlrStatsResult"),
		},
		"valid request with empty SPDK response": {
			testControllerName,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_get_ctrlr_stats: %v", "EOF"),
		},
		"valid request with ID mismatch SPDK response": {
			testControllerName,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_get_ctrlr_stats: %v", "json response ID mismatch"),
		},
		"valid request with error code from SPDK response": {
			testControllerName,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_get_ctrlr_stats: %v", "json response error: myopierr"),
		},
		"valid request with valid SPDK response": {
			testControllerName,
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
		},
		"valid request with unknown key": {
			server.ResourceIDToVolumeName("unknown-controller-id"),
			nil,
			[]string{},
			codes.Unknown,
			fmt.Sprintf("error finding controller %v", server.ResourceIDToVolumeName("unknown-controller-id")),
		},
		"malformed name": {
			"-ABC-DEF",
			nil,
			[]string{},
			codes.Unknown,
			fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Subsystems[testSubsystemName] = &testSubsystem
			testEnv.opiSpdkServer.Controllers[testControllerName] = &testController
			testEnv.opiSpdkServer.Namespaces[testNamespaceName] = &testNamespace

			request := &pb.NvmeControllerStatsRequest{Name: tt.in}
			response, err := testEnv.client.NvmeControllerStats(testEnv.ctx, request)

			if !proto.Equal(response.GetStats(), tt.out) {
				t.Error("response: expected", tt.out, "received", response.GetStats())
			}

			if er, ok := status.FromError(err); ok {
				if er.Code() != tt.errCode {
					t.Error("error code: expected", tt.errCode, "received", er.Code())
				}
				if er.Message() != tt.errMsg {
					t.Error("error message: expected", tt.errMsg, "received", er.Message())
				}
			} else {
				t.Error("expected grpc error status")
			}
		})
	}
}
