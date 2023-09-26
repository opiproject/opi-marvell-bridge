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
	"google.golang.org/protobuf/types/known/wrapperspb"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/frontend"
	server "github.com/opiproject/opi-spdk-bridge/pkg/utils"
)

func TestFrontEnd_CreateNvmeController(t *testing.T) {
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	spec := &pb.NvmeControllerSpec{
		PcieId: &pb.PciEndpoint{
			PhysicalFunction: wrapperspb.Int32(1),
			VirtualFunction:  wrapperspb.Int32(2),
			PortId:           wrapperspb.Int32(3)},
		NvmeControllerId: proto.Int32(1),
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
		subsys  string
	}{
		"illegal resource_id": {
			id: "CapitalLettersNotAllowed",
			in: &pb.NvmeController{
				Spec: spec,
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("user-settable ID must only contain lowercase, numbers and hyphens (%v)", "got: 'C' in position 0"),
			exist:   false,
			subsys:  testSubsystemName,
		},
		"valid request with invalid SPDK response": {
			id: testControllerID,
			in: &pb.NvmeController{
				Name: testControllerName,
				Spec: spec,
			},
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1, "ctrlr_id": -1}}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("Could not create CTRL: %v", testControllerName),
			exist:   false,
			subsys:  testSubsystemName,
		},
		"valid request with empty SPDK response": {
			id: testControllerID,
			in: &pb.NvmeController{
				Name: testControllerName,
				Spec: spec,
			},
			out:     nil,
			spdk:    []string{""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_subsys_create_ctrlr: %v", "EOF"),
			exist:   false,
			subsys:  testSubsystemName,
		},
		"valid request with ID mismatch SPDK response": {
			id: testControllerID,
			in: &pb.NvmeController{
				Name: testControllerName,
				Spec: spec,
			},
			out:     nil,
			spdk:    []string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1, "ctrlr_id": 17}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_subsys_create_ctrlr: %v", "json response ID mismatch"),
			exist:   false,
			subsys:  testSubsystemName,
		},
		"valid request with error code from SPDK response": {
			id: testControllerID,
			in: &pb.NvmeController{
				Name: testControllerName,
				Spec: spec,
			},
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":-32602,"message":"Invalid parameters"}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_subsys_create_ctrlr: %v", "json response error: Invalid parameters"),
			exist:   false,
			subsys:  testSubsystemName,
		},
		"valid request with valid SPDK response": {
			id: testControllerID,
			in: &pb.NvmeController{
				Name: testControllerName,
				Spec: &pb.NvmeControllerSpec{
					PcieId:           testController.Spec.PcieId,
					NvmeControllerId: proto.Int32(17),
					MaxNsq:           5,
					MaxNcq:           6,
					Sqes:             7,
					Cqes:             8,
				},
			},
			out: &pb.NvmeController{
				Name: testControllerName,
				Spec: &pb.NvmeControllerSpec{
					PcieId:           testController.Spec.PcieId,
					NvmeControllerId: proto.Int32(17),
					MaxNsq:           5,
					MaxNcq:           6,
					Sqes:             7,
					Cqes:             8,
				},
				Status: &pb.NvmeControllerStatus{Active: true},
			},
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0, "ctrlr_id": 17}}`},
			errCode: codes.OK,
			errMsg:  "",
			exist:   false,
			subsys:  testSubsystemName,
		},
		"already exists": {
			id: testControllerID,
			in: &pb.NvmeController{
				Name: testControllerName,
				Spec: spec,
			},
			out:     &testController,
			spdk:    []string{},
			errCode: codes.OK,
			errMsg:  "",
			exist:   true,
			subsys:  testSubsystemName,
		},
		"malformed subsystem name": {
			id: testControllerID,
			in: &pb.NvmeController{
				Spec: &pb.NvmeControllerSpec{
					PcieId:           testController.Spec.PcieId,
					NvmeControllerId: proto.Int32(1),
				},
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
			exist:   false,
			subsys:  "-ABC-DEF",
		},
		"no required ctrl field": {
			id:      testControllerID,
			in:      nil,
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  "missing required field: nvme_controller",
			exist:   false,
			subsys:  testSubsystemName,
		},
		"no required parent field": {
			id: testControllerID,
			in: &pb.NvmeController{
				Spec: &pb.NvmeControllerSpec{
					NvmeControllerId: proto.Int32(1),
				},
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  "missing required field: parent",
			exist:   false,
			subsys:  "",
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Subsystems[testSubsystemName] = server.ProtoClone(&testSubsystem)
			testEnv.opiSpdkServer.Subsystems[testSubsystemName].Name = testSubsystemName
			testEnv.opiSpdkServer.Namespaces[testNamespaceName] = server.ProtoClone(&testNamespace)
			testEnv.opiSpdkServer.Namespaces[testNamespaceName].Name = testNamespaceName
			if tt.exist {
				testEnv.opiSpdkServer.Controllers[testControllerName] = server.ProtoClone(&testController)
				testEnv.opiSpdkServer.Controllers[testControllerName].Name = testControllerName
				// testEnv.opiSpdkServer.Controllers[testControllerID].Spec.Id = &pc.ObjectKey{Value: testControllerID}
			}
			if tt.out != nil {
				tt.out = server.ProtoClone(tt.out)
				tt.out.Name = testControllerName
			}

			request := &pb.CreateNvmeControllerRequest{Parent: tt.subsys,
				NvmeController: tt.in, NvmeControllerId: tt.id}
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
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		in      string
		out     *emptypb.Empty
		spdk    []string
		errCode codes.Code
		errMsg  string
		missing bool
	}{
		"valid request with invalid SPDK response": {
			in:      testControllerName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("Could not delete CTRL: %v", testControllerName),
			missing: false,
		},
		"valid request with empty SPDK response": {
			in:      testControllerName,
			out:     nil,
			spdk:    []string{""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_subsys_remove_ctrlr: %v", "EOF"),
			missing: false,
		},
		"valid request with ID mismatch SPDK response": {
			in:      testControllerName,
			out:     nil,
			spdk:    []string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_subsys_remove_ctrlr: %v", "json response ID mismatch"),
			missing: false,
		},
		"valid request with error code from SPDK response": {
			in:      testControllerName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":{"status": 1}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_subsys_remove_ctrlr: %v", "json response error: myopierr"),
			missing: false,
		},
		"valid request with valid SPDK response": {
			in:      testControllerName,
			out:     &emptypb.Empty{},
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0}}`},
			errCode: codes.OK,
			errMsg:  "",
			missing: false,
		},
		"valid request with unknown key": {
			in:      frontend.ResourceIDToControllerName(testSubsystemID, "unknown-controller-id"),
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("error finding controller %v", frontend.ResourceIDToControllerName(testSubsystemID, "unknown-controller-id")),
			missing: false,
		},
		"unknown key with missing allowed": {
			in:      "unknown-id",
			out:     &emptypb.Empty{},
			spdk:    []string{},
			errCode: codes.OK,
			errMsg:  "",
			missing: true,
		},
		"malformed name": {
			in:      "-ABC-DEF",
			out:     &emptypb.Empty{},
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
			missing: false,
		},
		"no required field": {
			in:      "",
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  "missing required field: name",
			missing: false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Subsystems[testSubsystemName] = server.ProtoClone(&testSubsystem)
			testEnv.opiSpdkServer.Subsystems[testSubsystemName].Name = testSubsystemName
			testEnv.opiSpdkServer.Controllers[testControllerName] = server.ProtoClone(&testController)
			testEnv.opiSpdkServer.Controllers[testControllerName].Name = testControllerName
			testEnv.opiSpdkServer.Namespaces[testNamespaceName] = server.ProtoClone(&testNamespace)
			testEnv.opiSpdkServer.Namespaces[testNamespaceName].Name = testNamespaceName

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
		PcieId:           testController.Spec.PcieId,
		NvmeControllerId: proto.Int32(1),
		MaxNsq:           5,
		MaxNcq:           6,
		Sqes:             7,
		Cqes:             8,
	}
	t.Cleanup(server.CheckTestProtoObjectsNotChanged(spec)(t, t.Name()))
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))

	tests := map[string]struct {
		mask    *fieldmaskpb.FieldMask
		in      *pb.NvmeController
		out     *pb.NvmeController
		spdk    []string
		errCode codes.Code
		errMsg  string
	}{
		"invalid fieldmask": {
			mask: &fieldmaskpb.FieldMask{Paths: []string{"*", "author"}},
			in: &pb.NvmeController{
				Name: testControllerName,
				Spec: spec,
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("invalid field path: %s", "'*' must not be used with other paths"),
		},
		"valid request with invalid SPDK response": {
			mask: nil,
			in: &pb.NvmeController{
				Name: testControllerName,
				Spec: spec,
			},
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1, "ctrlr_id": -1}}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("Could not update CTRL: %v", testControllerName),
		},
		"valid request with empty SPDK response": {
			mask: nil,
			in: &pb.NvmeController{
				Name: testControllerName,
				Spec: spec,
			},
			out:     nil,
			spdk:    []string{""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_subsys_update_ctrlr: %v", "EOF"),
		},
		"valid request with ID mismatch SPDK response": {
			mask: nil,
			in: &pb.NvmeController{
				Name: testControllerName,
				Spec: spec,
			},
			out:     nil,
			spdk:    []string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1, "ctrlr_id": 17}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_subsys_update_ctrlr: %v", "json response ID mismatch"),
		},
		"valid request with error code from SPDK response": {
			mask: nil,
			in: &pb.NvmeController{
				Name: testControllerName,
				Spec: spec,
			},
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":-32602,"message":"Invalid parameters"}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_subsys_update_ctrlr: %v", "json response error: Invalid parameters"),
		},
		"valid request with valid SPDK response": {
			mask: nil,
			in: &pb.NvmeController{
				Name: testControllerName,
				Spec: &pb.NvmeControllerSpec{
					PcieId:           testController.Spec.PcieId,
					NvmeControllerId: proto.Int32(17),
					MaxNsq:           5,
					MaxNcq:           6,
					Sqes:             7,
					Cqes:             8,
				},
			},
			out: &pb.NvmeController{
				Name: testControllerName,
				Spec: &pb.NvmeControllerSpec{
					PcieId:           testController.Spec.PcieId,
					NvmeControllerId: proto.Int32(17),
					MaxNsq:           5,
					MaxNcq:           6,
					Sqes:             7,
					Cqes:             8,
				},
				Status: &pb.NvmeControllerStatus{Active: true},
			},
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0, "ctrlr_id": 17}}`},
			errCode: codes.OK,
			errMsg:  "",
		},
		"valid request with unknown key": {
			mask: nil,
			in: &pb.NvmeController{
				Name: frontend.ResourceIDToControllerName(testSubsystemID, "unknown-controller-id"),
				Spec: spec,
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.NotFound,
			errMsg:  fmt.Sprintf("unable to find key %v", frontend.ResourceIDToControllerName(testSubsystemID, "unknown-controller-id")),
		},
		"malformed name": {
			mask: nil,
			in: &pb.NvmeController{
				Name: "-ABC-DEF",
				Spec: spec,
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Subsystems[testSubsystemName] = server.ProtoClone(&testSubsystem)
			testEnv.opiSpdkServer.Subsystems[testSubsystemName].Name = testSubsystemName
			testEnv.opiSpdkServer.Controllers[testControllerName] = server.ProtoClone(&testController)
			testEnv.opiSpdkServer.Controllers[testControllerName].Name = testControllerName
			testEnv.opiSpdkServer.Namespaces[testNamespaceName] = server.ProtoClone(&testNamespace)
			testEnv.opiSpdkServer.Namespaces[testNamespaceName].Name = testNamespaceName

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
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
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
			in:      testSubsystemName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1, "ctrlr_id_list": []}}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("Could not list CTRLs: %v", testSubsystemName),
			size:    0,
			token:   "",
		},
		"valid request with empty SPDK response": {
			in:      testSubsystemName,
			out:     nil,
			spdk:    []string{""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_subsys_get_ctrlr_list: %v", "EOF"),
			size:    0,
			token:   "",
		},
		"valid request with ID mismatch SPDK response": {
			in:      testSubsystemName,
			out:     nil,
			spdk:    []string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1, "ctrlr_id_list": []}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_subsys_get_ctrlr_list: %v", "json response ID mismatch"),
			size:    0,
			token:   "",
		},
		"valid request with error code from SPDK response": {
			in:      testSubsystemName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":{"status": 1, "ctrlr_id_list": []}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_subsys_get_ctrlr_list: %v", "json response error: myopierr"),
			size:    0,
			token:   "",
		},
		"pagination negative": {
			in:      testSubsystemName,
			out:     nil,
			spdk:    []string{},
			errCode: codes.InvalidArgument,
			errMsg:  "negative PageSize is not allowed",
			size:    -10,
			token:   "",
		},
		"pagination error": {
			in:      testSubsystemName,
			out:     nil,
			spdk:    []string{},
			errCode: codes.NotFound,
			errMsg:  fmt.Sprintf("unable to find pagination token %s", "unknown-pagination-token"),
			size:    0,
			token:   "unknown-pagination-token",
		},
		"pagination": {
			in: testSubsystemName,
			out: []*pb.NvmeController{
				{
					Spec: &pb.NvmeControllerSpec{
						NvmeControllerId: proto.Int32(1),
					},
				},
			},
			spdk:    []string{`{"jsonrpc":"2.0","id":%d,"error":{"code":0,"message":""},"result":{"status":0,"ctrlr_id_list":[{"ctrlr_id":1},{"ctrlr_id":2},{"ctrlr_id":3}]}}`},
			errCode: codes.OK,
			errMsg:  "",
			size:    1,
			token:   "",
		},
		"pagination overflow": {
			in: testSubsystemName,
			out: []*pb.NvmeController{
				{
					Spec: &pb.NvmeControllerSpec{
						NvmeControllerId: proto.Int32(1),
					},
				},
				{
					Spec: &pb.NvmeControllerSpec{
						NvmeControllerId: proto.Int32(2),
					},
				},
				{
					Spec: &pb.NvmeControllerSpec{
						NvmeControllerId: proto.Int32(3),
					},
				},
			},
			spdk:    []string{`{"jsonrpc":"2.0","id":%d,"error":{"code":0,"message":""},"result":{"status":0,"ctrlr_id_list":[{"ctrlr_id":1},{"ctrlr_id":2},{"ctrlr_id":3}]}}`},
			errCode: codes.OK,
			errMsg:  "",
			size:    1000,
			token:   "",
		},
		"valid request with valid SPDK response": {
			in: testSubsystemName,
			out: []*pb.NvmeController{
				{
					Spec: &pb.NvmeControllerSpec{
						NvmeControllerId: proto.Int32(1),
					},
				},
				{
					Spec: &pb.NvmeControllerSpec{
						NvmeControllerId: proto.Int32(2),
					},
				},
				{
					Spec: &pb.NvmeControllerSpec{
						NvmeControllerId: proto.Int32(3),
					},
				},
			},
			spdk:    []string{`{"jsonrpc":"2.0","id":%d,"error":{"code":0,"message":""},"result":{"status":0,"ctrlr_id_list":[{"ctrlr_id":1},{"ctrlr_id":2},{"ctrlr_id":3}]}}`},
			errCode: codes.OK,
			errMsg:  "",
			size:    0,
			token:   "",
		},
		"valid request with unknown key": {
			in:      "unknown-subsystem-id",
			out:     nil,
			spdk:    []string{},
			errCode: codes.NotFound,
			errMsg:  fmt.Sprintf("unable to find key %v", "unknown-subsystem-id"),
			size:    0,
			token:   "",
		},
		"no required field": {
			in:      "",
			out:     []*pb.NvmeController{},
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  "missing required field: parent",
			size:    0,
			token:   "",
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Subsystems[testSubsystemName] = server.ProtoClone(&testSubsystem)
			testEnv.opiSpdkServer.Subsystems[testSubsystemName].Name = testSubsystemName
			testEnv.opiSpdkServer.Controllers[testControllerName] = server.ProtoClone(&testController)
			testEnv.opiSpdkServer.Controllers[testControllerName].Name = testControllerName
			testEnv.opiSpdkServer.Namespaces[testNamespaceName] = server.ProtoClone(&testNamespace)
			testEnv.opiSpdkServer.Namespaces[testNamespaceName].Name = testNamespaceName
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
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		in      string
		out     *pb.NvmeController
		spdk    []string
		errCode codes.Code
		errMsg  string
	}{
		"valid request with invalid SPDK response": {
			in:      testControllerName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status":1}}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("Could not get CTRL: %v", testControllerName),
		},
		"valid request with empty SPDK response": {
			in:      testControllerName,
			out:     nil,
			spdk:    []string{""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_ctrlr_get_info: %v", "EOF"),
		},
		"valid request with ID mismatch SPDK response": {
			in:      testControllerName,
			out:     nil,
			spdk:    []string{`{"id":0,"error":{"code":0,"message":""},"result":{"status":1}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_ctrlr_get_info: %v", "json response ID mismatch"),
		},
		"valid request with error code from SPDK response": {
			in:      testControllerName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":{"status":1}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_ctrlr_get_info: %v", "json response error: myopierr"),
		},
		"valid request with valid SPDK response": {
			in: testControllerName,
			out: &pb.NvmeController{
				Name: testControllerName,
				Spec: &pb.NvmeControllerSpec{
					NvmeControllerId: proto.Int32(17),
				},
				Status: &pb.NvmeControllerStatus{Active: true},
			},
			spdk:    []string{`{"jsonrpc":"2.0","id":%d,"result":{"status":0,"pcie_domain_id":1,"pf_id":1,"vf_id":1,"ctrlr_id":1,"max_nsq":4,"max_ncq":4,"mqes":2048,"ieee_oui":"005043","cmic":6,"nn":16,"active_ns_count":4,"active_nsq":2,"active_ncq":2,"mdts":9,"sqes":6,"cqes":4}}`},
			errCode: codes.OK,
			errMsg:  "",
		},
		"valid request with unknown key": {
			in:      frontend.ResourceIDToControllerName(testSubsystemID, "unknown-controller-id"),
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("error finding controller %v", frontend.ResourceIDToControllerName(testSubsystemID, "unknown-controller-id")),
		},
		"malformed name": {
			in:      "-ABC-DEF",
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
		},
		"no required field": {
			in:      "",
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  "missing required field: name",
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Subsystems[testSubsystemName] = server.ProtoClone(&testSubsystem)
			testEnv.opiSpdkServer.Subsystems[testSubsystemName].Name = testSubsystemName
			testEnv.opiSpdkServer.Controllers[testControllerName] = server.ProtoClone(&testController)
			testEnv.opiSpdkServer.Controllers[testControllerName].Name = testControllerName
			testEnv.opiSpdkServer.Namespaces[testNamespaceName] = server.ProtoClone(&testNamespace)
			testEnv.opiSpdkServer.Namespaces[testNamespaceName].Name = testNamespaceName

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

func TestFrontEnd_StatsNvmeController(t *testing.T) {
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		in      string
		out     *pb.VolumeStats
		spdk    []string
		errCode codes.Code
		errMsg  string
	}{
		"valid request with invalid SPDK response": {
			in:      testControllerName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("Could not stats CTRL: %v", testControllerName),
		},
		"valid request with invalid marshal SPDK response": {
			in:      testControllerName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_get_ctrlr_stats: %v", "json: cannot unmarshal array into Go value of type models.MrvlNvmGetCtrlrStatsResult"),
		},
		"valid request with empty SPDK response": {
			in:      testControllerName,
			out:     nil,
			spdk:    []string{""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_get_ctrlr_stats: %v", "EOF"),
		},
		"valid request with ID mismatch SPDK response": {
			in:      testControllerName,
			out:     nil,
			spdk:    []string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_get_ctrlr_stats: %v", "json response ID mismatch"),
		},
		"valid request with error code from SPDK response": {
			in:      testControllerName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_get_ctrlr_stats: %v", "json response error: myopierr"),
		},
		"valid request with valid SPDK response": {
			in: testControllerName,
			out: &pb.VolumeStats{
				ReadBytesCount:    5,
				ReadOpsCount:      4,
				WriteBytesCount:   7,
				WriteOpsCount:     6,
				ReadLatencyTicks:  9,
				WriteLatencyTicks: 10,
			},
			spdk:    []string{`{"jsonrpc":"2.0","id":%d,"result":{"status":0,"num_admin_cmds":1,"num_admin_cmd_errors":2,"num_async_events":3,"num_read_cmds":4,"num_read_bytes":5,"num_write_cmds":6,"num_write_bytes":7,"num_errors":8,"total_read_latency_in_us":9,"total_write_latency_in_us":10,"Stats_time_window_in_us":11}}`},
			errCode: codes.OK,
			errMsg:  "",
		},
		"valid request with unknown key": {
			in:      frontend.ResourceIDToControllerName(testSubsystemID, "unknown-controller-id"),
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("error finding controller %v", frontend.ResourceIDToControllerName(testSubsystemID, "unknown-controller-id")),
		},
		"malformed name": {
			in:      "-ABC-DEF",
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Subsystems[testSubsystemName] = server.ProtoClone(&testSubsystem)
			testEnv.opiSpdkServer.Subsystems[testSubsystemName].Name = testSubsystemName
			testEnv.opiSpdkServer.Controllers[testControllerName] = server.ProtoClone(&testController)
			testEnv.opiSpdkServer.Controllers[testControllerName].Name = testControllerName
			testEnv.opiSpdkServer.Namespaces[testNamespaceName] = server.ProtoClone(&testNamespace)
			testEnv.opiSpdkServer.Namespaces[testNamespaceName].Name = testNamespaceName

			request := &pb.StatsNvmeControllerRequest{Name: tt.in}
			response, err := testEnv.client.StatsNvmeController(testEnv.ctx, request)

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
