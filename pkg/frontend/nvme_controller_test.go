// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2022 Marvell International Ltd.
// Copyright (C) 2023 Intel Corporation

// Package frontend implememnts the FrontEnd APIs (host facing) of the storage Server
package frontend

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/server"
)

func TestFrontEnd_CreateNvmeController(t *testing.T) {
	spec := &pb.NvmeControllerSpec{
		SubsystemId:      &pc.ObjectKey{Value: testSubsystemName},
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
		start   bool
		exist   bool
	}{
		"illegal resource_id": {
			"CapitalLettersNotAllowed",
			&pb.NvmeController{
				Spec: spec,
			},
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("user-settable ID must only contain lowercase, numbers and hyphens (%v)", "got: 'C' in position 0"),
			false,
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
			true,
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
			true,
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
			true,
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
			true,
			false,
		},
		"valid request with valid SPDK response": {
			testControllerID,
			&pb.NvmeController{
				Name: testControllerName,
				Spec: &pb.NvmeControllerSpec{
					SubsystemId:      &pc.ObjectKey{Value: testSubsystemName},
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
					SubsystemId: &pc.ObjectKey{Value: testSubsystemName},
					PcieId:      &pb.PciEndpoint{PhysicalFunction: 1, VirtualFunction: 2, PortId: 3},
					MaxNsq:      5,
					MaxNcq:      6,
					Sqes:        7,
					Cqes:        8,
				},
				Status: &pb.NvmeControllerStatus{Active: true},
			},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0, "cntlid": 17}}`},
			codes.OK,
			"",
			true,
			false,
		},
		"already exists": {
			testControllerID,
			&pb.NvmeController{
				Name: testControllerName,
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
						t.Error("error code: expected", tt.errCode, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
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
		start   bool
		missing bool
	}{
		"valid request with invalid SPDK response": {
			testControllerName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not delete CTRL: %v", testControllerName),
			true,
			false,
		},
		"valid request with empty SPDK response": {
			testControllerName,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_remove_ctrlr: %v", "EOF"),
			true,
			false,
		},
		"valid request with ID mismatch SPDK response": {
			testControllerName,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_remove_ctrlr: %v", "json response ID mismatch"),
			true,
			false,
		},
		"valid request with error code from SPDK response": {
			testControllerName,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_remove_ctrlr: %v", "json response error: myopierr"),
			true,
			false,
		},
		"valid request with valid SPDK response": {
			testControllerName,
			&emptypb.Empty{},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0}}`},
			codes.OK,
			"",
			true,
			false,
		},
		"valid request with unknown key": {
			server.ResourceIDToVolumeName("unknown-controller-id"),
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("error finding controller %v", server.ResourceIDToVolumeName("unknown-controller-id")),
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

			testEnv.opiSpdkServer.Subsystems[testSubsystemName] = &testSubsystem
			testEnv.opiSpdkServer.Controllers[testControllerName] = &testController
			testEnv.opiSpdkServer.Namespaces[testNamespaceName] = &testNamespace

			request := &pb.DeleteNvmeControllerRequest{Name: tt.in, AllowMissing: tt.missing}
			response, err := testEnv.client.DeleteNvmeController(testEnv.ctx, request)
			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", tt.errCode, "received", er.Code())
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

func TestFrontEnd_UpdateNvmeController(t *testing.T) {
	spec := &pb.NvmeControllerSpec{
		SubsystemId:      &pc.ObjectKey{Value: testSubsystemName},
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
		start   bool
	}{
		"invalid fieldmask": {
			&fieldmaskpb.FieldMask{Paths: []string{"*", "author"}},
			&pb.NvmeController{
				Name: testControllerName,
				Spec: spec,
			},
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("invalid field path: %s", "'*' must not be used with other paths"),
			false,
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
			true,
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
			true,
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
			true,
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
			true,
		},
		"valid request with valid SPDK response": {
			nil,
			&pb.NvmeController{
				Name: testControllerName,
				Spec: &pb.NvmeControllerSpec{
					SubsystemId:      &pc.ObjectKey{Value: testSubsystemName},
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
					SubsystemId: &pc.ObjectKey{Value: testSubsystemName},
					PcieId:      &pb.PciEndpoint{PhysicalFunction: 1, VirtualFunction: 2, PortId: 3},
					MaxNsq:      5,
					MaxNcq:      6,
					Sqes:        7,
					Cqes:        8,
				},
				Status: &pb.NvmeControllerStatus{Active: true},
			},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0, "cntlid": 17}}`},
			codes.OK,
			"",
			true,
		},
		"valid request with unknown key": {
			nil,
			&pb.NvmeController{
				Name: server.ResourceIDToVolumeName("unknown-id"),
			},
			nil,
			[]string{""},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", server.ResourceIDToVolumeName("unknown-id")),
			false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Subsystems[testSubsystemName] = &testSubsystem
			testEnv.opiSpdkServer.Controllers[testControllerName] = &testController
			testEnv.opiSpdkServer.Namespaces[testNamespaceName] = &testNamespace

			request := &pb.UpdateNvmeControllerRequest{NvmeController: tt.in, UpdateMask: tt.mask}
			response, err := testEnv.client.UpdateNvmeController(testEnv.ctx, request)
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
						t.Error("error code: expected", tt.errCode, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
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
		start   bool
		size    int32
		token   string
	}{
		"valid request with invalid SPDK response": {
			testSubsystemName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1, "ctrlr_id_list": []}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not list CTRLs: %v", testSubsystemName),
			true,
			0,
			"",
		},
		"valid request with empty SPDK response": {
			testSubsystemName,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_get_ctrlr_list: %v", "EOF"),
			true,
			0,
			"",
		},
		"valid request with ID mismatch SPDK response": {
			testSubsystemName,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1, "ctrlr_id_list": []}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_get_ctrlr_list: %v", "json response ID mismatch"),
			true,
			0,
			"",
		},
		"valid request with error code from SPDK response": {
			testSubsystemName,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":{"status": 1, "ctrlr_id_list": []}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_get_ctrlr_list: %v", "json response error: myopierr"),
			true,
			0,
			"",
		},
		"pagination negative": {
			testSubsystemName,
			nil,
			[]string{},
			codes.InvalidArgument,
			"negative PageSize is not allowed",
			false,
			-10,
			"",
		},
		"pagination error": {
			testSubsystemName,
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find pagination token %s", "unknown-pagination-token"),
			false,
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
			true,
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
			true,
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

			testEnv.opiSpdkServer.Subsystems[testSubsystemName] = &testSubsystem
			testEnv.opiSpdkServer.Controllers[testControllerName] = &testController
			testEnv.opiSpdkServer.Namespaces[testNamespaceName] = &testNamespace
			testEnv.opiSpdkServer.Pagination["existing-pagination-token"] = 1

			request := &pb.ListNvmeControllersRequest{Parent: tt.in, PageSize: tt.size, PageToken: tt.token}
			response, err := testEnv.client.ListNvmeControllers(testEnv.ctx, request)
			if response != nil {
				if !reflect.DeepEqual(response.NvmeControllers, tt.out) {
					t.Error("response: expected", tt.out, "received", response.NvmeControllers)
				}
			}

			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", tt.errCode, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
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
		start   bool
	}{
		"valid request with invalid SPDK response": {
			testControllerName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status":1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not get CTRL: %v", testControllerName),
			true,
		},
		"valid request with empty SPDK response": {
			testControllerName,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ctrlr_get_info: %v", "EOF"),
			true,
		},
		"valid request with ID mismatch SPDK response": {
			testControllerName,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status":1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ctrlr_get_info: %v", "json response ID mismatch"),
			true,
		},
		"valid request with error code from SPDK response": {
			testControllerName,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":{"status":1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ctrlr_get_info: %v", "json response error: myopierr"),
			true,
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
			true,
		},
		"valid request with unknown key": {
			server.ResourceIDToVolumeName("unknown-subsystem-id"),
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("error finding controller %v", server.ResourceIDToVolumeName("unknown-subsystem-id")),
			false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Subsystems[testSubsystemName] = &testSubsystem
			testEnv.opiSpdkServer.Controllers[testControllerName] = &testController
			testEnv.opiSpdkServer.Namespaces[testNamespaceName] = &testNamespace

			request := &pb.GetNvmeControllerRequest{Name: tt.in}
			response, err := testEnv.client.GetNvmeController(testEnv.ctx, request)
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
						t.Error("error code: expected", tt.errCode, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
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
		start   bool
	}{
		"valid request with invalid SPDK response": {
			testControllerName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not stats CTRL: %v", testControllerName),
			true,
		},
		"valid request with invalid marshal SPDK response": {
			testControllerName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_get_ctrlr_stats: %v", "json: cannot unmarshal array into Go value of type models.MrvlNvmGetCtrlrStatsResult"),
			true,
		},
		"valid request with empty SPDK response": {
			testControllerName,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_get_ctrlr_stats: %v", "EOF"),
			true,
		},
		"valid request with ID mismatch SPDK response": {
			testControllerName,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_get_ctrlr_stats: %v", "json response ID mismatch"),
			true,
		},
		"valid request with error code from SPDK response": {
			testControllerName,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_get_ctrlr_stats: %v", "json response error: myopierr"),
			true,
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
			true,
		},
		"valid request with unknown key": {
			server.ResourceIDToVolumeName("unknown-controller-id"),
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("error finding controller %v", server.ResourceIDToVolumeName("unknown-controller-id")),
			false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.start, tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Subsystems[testSubsystemName] = &testSubsystem
			testEnv.opiSpdkServer.Controllers[testControllerName] = &testController
			testEnv.opiSpdkServer.Namespaces[testNamespaceName] = &testNamespace

			request := &pb.NvmeControllerStatsRequest{Id: &pc.ObjectKey{Value: tt.in}}
			response, err := testEnv.client.NvmeControllerStats(testEnv.ctx, request)
			if response != nil {
				if !reflect.DeepEqual(response.Stats, tt.out) {
					t.Error("response: expected", tt.out, "received", response.Stats)
				}
			}

			if err != nil {
				if er, ok := status.FromError(err); ok {
					if er.Code() != tt.errCode {
						t.Error("error code: expected", tt.errCode, "received", er.Code())
					}
					if er.Message() != tt.errMsg {
						t.Error("error message: expected", tt.errMsg, "received", er.Message())
					}
				}
			}
		})
	}
}
