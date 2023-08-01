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

	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-spdk-bridge/pkg/server"
)

func TestFrontEnd_CreateNvmeNamespace(t *testing.T) {
	spec := &pb.NvmeNamespaceSpec{
		SubsystemNameRef: testSubsystemName,
		HostNsid:         0,
		VolumeNameRef:    "Malloc1",
		Uuid:             &pc.Uuid{Value: "1b4e28ba-2fa1-11d2-883f-b9a761bde3fb"},
		Nguid:            "1b4e28ba-2fa1-11d2-883f-b9a761bde3fb",
		Eui64:            1967554867335598546,
	}
	tests := map[string]struct {
		id      string
		in      *pb.NvmeNamespace
		out     *pb.NvmeNamespace
		spdk    []string
		errCode codes.Code
		errMsg  string
		exist   bool
	}{
		"illegal resource_id": {
			"CapitalLettersNotAllowed",
			&pb.NvmeNamespace{
				Spec: spec,
			},
			nil,
			[]string{},
			codes.Unknown,
			fmt.Sprintf("user-settable ID must only contain lowercase, numbers and hyphens (%v)", "got: 'C' in position 0"),
			false,
		},
		"valid request with invalid SPDK response": {
			testNamespaceID,
			&pb.NvmeNamespace{
				Name: testNamespaceName,
				Spec: spec,
			},
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not create NS: %v", testNamespaceName),
			false,
		},
		"valid request with empty SPDK response": {
			testNamespaceID,
			&pb.NvmeNamespace{
				Name: testNamespaceName,
				Spec: spec,
			},
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_alloc_ns: %v", "EOF"),
			false,
		},
		"valid request with ID mismatch SPDK response": {
			testNamespaceID,
			&pb.NvmeNamespace{
				Name: testNamespaceName,
				Spec: spec,
			},
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_alloc_ns: %v", "json response ID mismatch"),
			false,
		},
		"valid request with error code from SPDK response": {
			testNamespaceID,
			&pb.NvmeNamespace{
				Name: testNamespaceName,
				Spec: spec,
			},
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_alloc_ns: %v", "json response error: myopierr"),
			false,
		},
		"valid request with valid SPDK response": {
			testNamespaceID,
			&pb.NvmeNamespace{
				Name: testNamespaceName,
				Spec: &pb.NvmeNamespaceSpec{
					SubsystemNameRef: testSubsystemName,
					HostNsid:         22,
					VolumeNameRef:    "Malloc1",
					Uuid:             &pc.Uuid{Value: "1b4e28ba-2fa1-11d2-883f-b9a761bde3fb"},
					Nguid:            "1b4e28ba-2fa1-11d2-883f-b9a761bde3fb",
					Eui64:            1967554867335598546,
				},
			},
			&pb.NvmeNamespace{
				Name: testNamespaceName,
				Spec: &pb.NvmeNamespaceSpec{
					SubsystemNameRef: testSubsystemName,
					HostNsid:         22,
					VolumeNameRef:    "Malloc1",
					Uuid:             &pc.Uuid{Value: "1b4e28ba-2fa1-11d2-883f-b9a761bde3fb"},
					Nguid:            "1b4e28ba-2fa1-11d2-883f-b9a761bde3fb",
					Eui64:            1967554867335598546,
				},
				Status: &pb.NvmeNamespaceStatus{
					PciState:     2,
					PciOperState: 1,
				},
			},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0, "ns_instance_id": 17}}`, `{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0}}`},
			codes.OK,
			"",
			false,
		},
		"valid request with invalid SPDK second attach response": {
			testNamespaceID,
			&pb.NvmeNamespace{
				Name: testNamespaceName,
				Spec: spec,
			},
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0, "ns_instance_id": 17}}`, `{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not attach NS: %v", testNamespaceName),
			false,
		},
		"already exists": {
			testNamespaceID,
			&pb.NvmeNamespace{
				Name: testNamespaceName,
				Spec: spec,
			},
			&testNamespace,
			[]string{},
			codes.OK,
			"",
			true,
		},
		"no required field": {
			testControllerID,
			nil,
			nil,
			[]string{},
			codes.Unknown,
			"missing required field: nvme_namespace",
			false,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Subsystems[testSubsystemName] = &testSubsystem
			testEnv.opiSpdkServer.Subsystems[testSubsystemName].Name = testSubsystemName
			testEnv.opiSpdkServer.Controllers[testControllerName] = &testController
			if tt.exist {
				testEnv.opiSpdkServer.Namespaces[testNamespaceName] = &testNamespace
				testEnv.opiSpdkServer.Namespaces[testNamespaceName].Name = testNamespaceName
			}
			if tt.out != nil {
				tt.out.Name = testNamespaceName
			}

			request := &pb.CreateNvmeNamespaceRequest{NvmeNamespace: tt.in, NvmeNamespaceId: tt.id}
			response, err := testEnv.client.CreateNvmeNamespace(testEnv.ctx, request)

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

func TestFrontEnd_DeleteNvmeNamespace(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *emptypb.Empty
		spdk    []string
		errCode codes.Code
		errMsg  string
		missing bool
	}{
		"valid request with invalid SPDK response": {
			testNamespaceName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not detach NS: %v", testNamespaceName),
			false,
		},
		"valid request with invalid SPDK second response": {
			testNamespaceName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0}}`, `{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not delete NS: %v", testNamespaceName),
			false,
		},
		"valid request with empty SPDK response": {
			testNamespaceName,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ctrlr_detach_ns: %v", "EOF"),
			false,
		},
		"valid request with ID mismatch SPDK response": {
			testNamespaceName,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ctrlr_detach_ns: %v", "json response ID mismatch"),
			false,
		},
		"valid request with error code from SPDK response": {
			testNamespaceName,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ctrlr_detach_ns: %v", "json response error: myopierr"),
			false,
		},
		"valid request with valid SPDK response": {
			testNamespaceName,
			&emptypb.Empty{},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0}}`, `{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0}}`},
			codes.OK,
			"",
			false,
		},
		"valid request with unknown key": {
			server.ResourceIDToVolumeName("unknown-namespace-id"),
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", server.ResourceIDToVolumeName("unknown-namespace-id")),
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
		"no required field": {
			"",
			nil,
			[]string{},
			codes.Unknown,
			"missing required field: name",
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
			request := &pb.DeleteNvmeNamespaceRequest{Name: tt.in, AllowMissing: tt.missing}
			response, err := testEnv.client.DeleteNvmeNamespace(testEnv.ctx, request)

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

func TestFrontEnd_UpdateNvmeNamespace(t *testing.T) {
	tests := map[string]struct {
		mask    *fieldmaskpb.FieldMask
		in      *pb.NvmeNamespace
		out     *pb.NvmeNamespace
		spdk    []string
		errCode codes.Code
		errMsg  string
	}{
		"invalid fieldmask": {
			&fieldmaskpb.FieldMask{Paths: []string{"*", "author"}},
			&pb.NvmeNamespace{
				Name: testNamespaceName,
			},
			nil,
			[]string{},
			codes.Unknown,
			fmt.Sprintf("invalid field path: %s", "'*' must not be used with other paths"),
		},
		"unimplemented method": {
			nil,
			&pb.NvmeNamespace{
				Name: testNamespaceName,
			},
			nil,
			[]string{},
			codes.Unimplemented,
			fmt.Sprintf("%v method is not implemented", "UpdateNvmeNamespace"),
		},
		"valid request with unknown key": {
			nil,
			&pb.NvmeNamespace{
				Name: server.ResourceIDToVolumeName("unknown-id"),
			},
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", server.ResourceIDToVolumeName("unknown-id")),
		},
		"malformed name": {
			nil,
			&pb.NvmeNamespace{Name: "-ABC-DEF"},
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
			request := &pb.UpdateNvmeNamespaceRequest{NvmeNamespace: tt.in, UpdateMask: tt.mask}
			response, err := testEnv.client.UpdateNvmeNamespace(testEnv.ctx, request)

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

func TestFrontEnd_ListNvmeNamespaces(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     []*pb.NvmeNamespace
		spdk    []string
		errCode codes.Code
		errMsg  string
		size    int32
		token   string
	}{
		"valid request with invalid SPDK response": {
			testSubsystemName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status":1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not list NS: %v", testSubsystemName),
			0,
			"",
		},
		"valid request with invalid marshal SPDK response": {
			testSubsystemName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_get_ns_list: %v", "json: cannot unmarshal array into Go value of type models.MrvlNvmSubsysGetNsListResult"),
			0,
			"",
		},
		"valid request with empty SPDK response": {
			testSubsystemName,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_get_ns_list: %v", "EOF"),
			0,
			"",
		},
		"valid request with ID mismatch SPDK response": {
			testSubsystemName,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status":1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_get_ns_list: %v", "json response ID mismatch"),
			0,
			"",
		},
		"valid request with error code from SPDK response": {
			testSubsystemName,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_get_ns_list: %v", "json response error: myopierr"),
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
			[]*pb.NvmeNamespace{
				{
					Spec: &pb.NvmeNamespaceSpec{
						HostNsid: 11,
					},
				},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":{"status":0,"ns_list":[{"ns_instance_id":11,"bdev":"bdev01","ctrlr_id_list":[{"ctrlr_id":1}]},{"ns_instance_id":12,"bdev":"bdev02","ctrlr_id_list":[]},{"ns_instance_id":13,"bdev":"bdev03","ctrlr_id_list":[]}]}}`},
			codes.OK,
			"",
			1,
			"",
		},
		"pagination overflow": {
			testSubsystemName,
			[]*pb.NvmeNamespace{
				{
					Spec: &pb.NvmeNamespaceSpec{
						HostNsid: 11,
					},
				},
				{
					Spec: &pb.NvmeNamespaceSpec{
						HostNsid: 12,
					},
				},
				{
					Spec: &pb.NvmeNamespaceSpec{
						HostNsid: 13,
					},
				},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":{"status":0,"ns_list":[{"ns_instance_id":11,"bdev":"bdev01","ctrlr_id_list":[{"ctrlr_id":1}]},{"ns_instance_id":12,"bdev":"bdev02","ctrlr_id_list":[]},{"ns_instance_id":13,"bdev":"bdev03","ctrlr_id_list":[]}]}}`},
			codes.OK,
			"",
			1000,
			"",
		},
		"pagination offset": {
			testSubsystemName,
			[]*pb.NvmeNamespace{
				{
					Spec: &pb.NvmeNamespaceSpec{
						HostNsid: 12,
					},
				},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":{"status":0,"ns_list":[{"ns_instance_id":11,"bdev":"bdev01","ctrlr_id_list":[{"ctrlr_id":1}]},{"ns_instance_id":12,"bdev":"bdev02","ctrlr_id_list":[]},{"ns_instance_id":13,"bdev":"bdev03","ctrlr_id_list":[]}]}}`},
			codes.OK,
			"",
			1,
			"existing-pagination-token",
		},
		"valid request with valid SPDK response": {
			testSubsystemName,
			[]*pb.NvmeNamespace{
				{
					Spec: &pb.NvmeNamespaceSpec{
						HostNsid: 11,
					},
				},
				{
					Spec: &pb.NvmeNamespaceSpec{
						HostNsid: 12,
					},
				},
				{
					Spec: &pb.NvmeNamespaceSpec{
						HostNsid: 13,
					},
				},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":{"status":0,"ns_list":[{"ns_instance_id":11,"bdev":"bdev01","ctrlr_id_list":[{"ctrlr_id":1}]},{"ns_instance_id":12,"bdev":"bdev02","ctrlr_id_list":[]},{"ns_instance_id":13,"bdev":"bdev03","ctrlr_id_list":[]}]}}`},
			codes.OK,
			"",
			0,
			"",
		},
		"valid request with unknown key": {
			server.ResourceIDToVolumeName("unknown-namespace-id"),
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", server.ResourceIDToVolumeName("unknown-namespace-id")),
			0,
			"",
		},
		"no required field": {
			"",
			[]*pb.NvmeNamespace{},
			[]string{},
			codes.Unknown,
			"missing required field: parent",
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

			request := &pb.ListNvmeNamespacesRequest{Parent: tt.in, PageSize: tt.size, PageToken: tt.token}
			response, err := testEnv.client.ListNvmeNamespaces(testEnv.ctx, request)

			if !server.EqualProtoSlices(response.GetNvmeNamespaces(), tt.out) {
				t.Error("response: expected", tt.out, "received", response.GetNvmeNamespaces())
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

func TestFrontEnd_GetNvmeNamespace(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *pb.NvmeNamespace
		spdk    []string
		errCode codes.Code
		errMsg  string
	}{
		"valid request with invalid SPDK response": {
			testNamespaceName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status":1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not get NS: %v", testNamespaceName),
		},
		"valid request with invalid marshal SPDK response": {
			testNamespaceName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ns_get_info: %v", "json: cannot unmarshal array into Go value of type models.MrvlNvmGetNsInfoResult"),
		},
		"valid request with empty SPDK response": {
			testNamespaceName,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ns_get_info: %v", "EOF"),
		},
		"valid request with ID mismatch SPDK response": {
			testNamespaceName,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status":1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ns_get_info: %v", "json response ID mismatch"),
		},
		"valid request with error code from SPDK response": {
			testNamespaceName,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_ns_get_info: %v", "json response error: myopierr"),
		},
		"valid request with valid SPDK response": {
			testNamespaceName,
			&pb.NvmeNamespace{
				Name: testNamespaceName,
				Spec: &pb.NvmeNamespaceSpec{
					Nguid: "0x25f9cbc45d0f976fb9c1a14ff5aed4b0",
				},
				Status: &pb.NvmeNamespaceStatus{
					PciState:     2,
					PciOperState: 1,
				},
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":{"status":0,"nguid":"0x25f9cbc45d0f976fb9c1a14ff5aed4b0","eui64":"0xa7632f80702e4242","uuid":"0xb35633240b77073b8b4ebda571120dfb","nmic":1,"bdev":"bdev01","num_ctrlrs":1,"ctrlr_id_list":[{"ctrlr_id":1}]}}`},
			codes.OK,
			"",
		},
		"valid request with unknown key": {
			"unknown-namespace-id",
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", "unknown-namespace-id"),
		},
		"malformed name": {
			"-ABC-DEF",
			nil,
			[]string{},
			codes.Unknown,
			fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
		},
		"no required field": {
			"",
			nil,
			[]string{},
			codes.Unknown,
			"missing required field: name",
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Subsystems[testSubsystemName] = &testSubsystem
			testEnv.opiSpdkServer.Subsystems[testSubsystemName].Name = testSubsystemName
			testEnv.opiSpdkServer.Controllers[testControllerName] = &testController
			testEnv.opiSpdkServer.Controllers[testControllerName].Name = testControllerName
			testEnv.opiSpdkServer.Namespaces[testNamespaceName] = &testNamespace
			testEnv.opiSpdkServer.Namespaces[testNamespaceName].Name = testNamespaceID

			request := &pb.GetNvmeNamespaceRequest{Name: tt.in}
			response, err := testEnv.client.GetNvmeNamespace(testEnv.ctx, request)

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

func TestFrontEnd_NvmeNamespaceStats(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *pb.VolumeStats
		spdk    []string
		errCode codes.Code
		errMsg  string
	}{
		"valid request with invalid SPDK response": {
			testNamespaceName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not stats NS: %v", testNamespaceName),
		},
		"valid request with invalid marshal SPDK response": {
			testNamespaceName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_get_ns_stats: %v", "json: cannot unmarshal array into Go value of type models.MrvlNvmGetNsStatsResult"),
		},
		"valid request with empty SPDK response": {
			testNamespaceName,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_get_ns_stats: %v", "EOF"),
		},
		"valid request with ID mismatch SPDK response": {
			testNamespaceName,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_get_ns_stats: %v", "json response ID mismatch"),
		},
		"valid request with error code from SPDK response": {
			testNamespaceName,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_get_ns_stats: %v", "json response error: myopierr"),
		},
		"valid request with valid SPDK response": {
			testNamespaceName,
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
		},
		"valid request with unknown key": {
			"unknown-namespace-id",
			nil,
			[]string{},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", "unknown-namespace-id"),
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

			request := &pb.NvmeNamespaceStatsRequest{Name: tt.in}
			response, err := testEnv.client.NvmeNamespaceStats(testEnv.ctx, request)

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
