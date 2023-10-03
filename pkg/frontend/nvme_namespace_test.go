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
	"github.com/opiproject/opi-spdk-bridge/pkg/frontend"
	"github.com/opiproject/opi-spdk-bridge/pkg/utils"
)

func TestFrontEnd_CreateNvmeNamespace(t *testing.T) {
	spec := &pb.NvmeNamespaceSpec{
		HostNsid:      0,
		VolumeNameRef: "Malloc1",
		Uuid:          &pc.Uuid{Value: "1b4e28ba-2fa1-11d2-883f-b9a761bde3fb"},
		Nguid:         "1b4e28ba-2fa1-11d2-883f-b9a761bde3fb",
		Eui64:         1967554867335598546,
	}
	t.Cleanup(utils.CheckTestProtoObjectsNotChanged(spec)(t, t.Name()))
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))

	tests := map[string]struct {
		id      string
		in      *pb.NvmeNamespace
		out     *pb.NvmeNamespace
		spdk    []string
		errCode codes.Code
		errMsg  string
		exist   bool
		subsys  string
	}{
		"illegal resource_id": {
			id: "CapitalLettersNotAllowed",
			in: &pb.NvmeNamespace{
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
			id: testNamespaceID,
			in: &pb.NvmeNamespace{
				Name: testNamespaceName,
				Spec: spec,
			},
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("Could not create NS: %v", testNamespaceName),
			exist:   false,
			subsys:  testSubsystemName,
		},
		"valid request with empty SPDK response": {
			id: testNamespaceID,
			in: &pb.NvmeNamespace{
				Name: testNamespaceName,
				Spec: spec,
			},
			out:     nil,
			spdk:    []string{""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_subsys_alloc_ns: %v", "EOF"),
			exist:   false,
			subsys:  testSubsystemName,
		},
		"valid request with ID mismatch SPDK response": {
			id: testNamespaceID,
			in: &pb.NvmeNamespace{
				Name: testNamespaceName,
				Spec: spec,
			},
			out:     nil,
			spdk:    []string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_subsys_alloc_ns: %v", "json response ID mismatch"),
			exist:   false,
			subsys:  testSubsystemName,
		},
		"valid request with error code from SPDK response": {
			id: testNamespaceID,
			in: &pb.NvmeNamespace{
				Name: testNamespaceName,
				Spec: spec,
			},
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":{"status": 1}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_subsys_alloc_ns: %v", "json response error: myopierr"),
			exist:   false,
			subsys:  testSubsystemName,
		},
		"valid request with valid SPDK response": {
			id: testNamespaceID,
			in: &pb.NvmeNamespace{
				Name: testNamespaceName,
				Spec: &pb.NvmeNamespaceSpec{
					HostNsid:      22,
					VolumeNameRef: "Malloc1",
					Uuid:          &pc.Uuid{Value: "1b4e28ba-2fa1-11d2-883f-b9a761bde3fb"},
					Nguid:         "1b4e28ba-2fa1-11d2-883f-b9a761bde3fb",
					Eui64:         1967554867335598546,
				},
			},
			out: &pb.NvmeNamespace{
				Name: testNamespaceName,
				Spec: &pb.NvmeNamespaceSpec{
					HostNsid:      22,
					VolumeNameRef: "Malloc1",
					Uuid:          &pc.Uuid{Value: "1b4e28ba-2fa1-11d2-883f-b9a761bde3fb"},
					Nguid:         "1b4e28ba-2fa1-11d2-883f-b9a761bde3fb",
					Eui64:         1967554867335598546,
				},
				Status: &pb.NvmeNamespaceStatus{
					PciState:     2,
					PciOperState: 1,
				},
			},
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0, "ns_instance_id": 17}}`, `{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0}}`},
			errCode: codes.OK,
			errMsg:  "",
			exist:   false,
			subsys:  testSubsystemName,
		},
		"valid request with invalid SPDK second attach response": {
			id: testNamespaceID,
			in: &pb.NvmeNamespace{
				Name: testNamespaceName,
				Spec: spec,
			},
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0, "ns_instance_id": 17}}`, `{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("Could not attach NS: %v", testNamespaceName),
			exist:   false,
			subsys:  testSubsystemName,
		},
		"already exists": {
			id: testNamespaceID,
			in: &pb.NvmeNamespace{
				Name: testNamespaceName,
				Spec: spec,
			},
			out:     &testNamespaceWithStatus,
			spdk:    []string{},
			errCode: codes.OK,
			errMsg:  "",
			exist:   true,
			subsys:  testSubsystemName,
		},
		"malformed subsystem name": {
			id: testNamespaceID,
			in: &pb.NvmeNamespace{
				Spec: &pb.NvmeNamespaceSpec{
					VolumeNameRef: "TBD",
				},
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("segment '%s': not a valid DNS name", "-ABC-DEF"),
			exist:   false,
			subsys:  "-ABC-DEF",
		},
		"no required ns field": {
			id:      testNamespaceID,
			in:      nil,
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  "missing required field: nvme_namespace",
			exist:   false,
			subsys:  testSubsystemName,
		},
		"no required subsystem field": {
			id: testNamespaceID,
			in: &pb.NvmeNamespace{
				Spec: &pb.NvmeNamespaceSpec{
					VolumeNameRef: "TBD",
				},
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  "missing required field: parent",
			exist:   false,
			subsys:  "",
		},
		"no required volume field": {
			id: testNamespaceID,
			in: &pb.NvmeNamespace{
				Spec: &pb.NvmeNamespaceSpec{},
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  "missing required field: nvme_namespace.spec.volume_name_ref",
			exist:   false,
			subsys:  testSubsystemName,
		},
	}

	// run tests
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			testEnv := createTestEnvironment(tt.spdk)
			defer testEnv.Close()

			testEnv.opiSpdkServer.Subsystems[testSubsystemName] = utils.ProtoClone(&testSubsystemWithStatus)
			testEnv.opiSpdkServer.Controllers[testControllerName] = utils.ProtoClone(&testControllerWithStatus)
			if tt.exist {
				testEnv.opiSpdkServer.Namespaces[testNamespaceName] = utils.ProtoClone(&testNamespaceWithStatus)
			}
			if tt.out != nil {
				tt.out = utils.ProtoClone(tt.out)
				tt.out.Name = testNamespaceName
			}

			request := &pb.CreateNvmeNamespaceRequest{Parent: tt.subsys,
				NvmeNamespace: tt.in, NvmeNamespaceId: tt.id}
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
			in:      testNamespaceName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("Could not detach NS: %v", testNamespaceName),
			missing: false,
		},
		"valid request with invalid SPDK second response": {
			in:      testNamespaceName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0}}`, `{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("Could not delete NS: %v", testNamespaceName),
			missing: false,
		},
		"valid request with empty SPDK response": {
			in:      testNamespaceName,
			out:     nil,
			spdk:    []string{""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_ctrlr_detach_ns: %v", "EOF"),
			missing: false,
		},
		"valid request with ID mismatch SPDK response": {
			in:      testNamespaceName,
			out:     nil,
			spdk:    []string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_ctrlr_detach_ns: %v", "json response ID mismatch"),
			missing: false,
		},
		"valid request with error code from SPDK response": {
			in:      testNamespaceName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":{"status": 1}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_ctrlr_detach_ns: %v", "json response error: myopierr"),
			missing: false,
		},
		"valid request with valid SPDK response": {
			in:      testNamespaceName,
			out:     &emptypb.Empty{},
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0}}`, `{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0}}`},
			errCode: codes.OK,
			errMsg:  "",
			missing: false,
		},
		"valid request with unknown key": {
			in:      frontend.ResourceIDToNamespaceName(testSubsystemID, "unknown-namespace-id"),
			out:     nil,
			spdk:    []string{},
			errCode: codes.NotFound,
			errMsg:  fmt.Sprintf("unable to find key %v", frontend.ResourceIDToNamespaceName(testSubsystemID, "unknown-namespace-id")),
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

			testEnv.opiSpdkServer.Subsystems[testSubsystemName] = utils.ProtoClone(&testSubsystemWithStatus)
			testEnv.opiSpdkServer.Controllers[testControllerName] = utils.ProtoClone(&testControllerWithStatus)
			testEnv.opiSpdkServer.Namespaces[testNamespaceName] = utils.ProtoClone(&testNamespaceWithStatus)
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
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		mask    *fieldmaskpb.FieldMask
		in      *pb.NvmeNamespace
		out     *pb.NvmeNamespace
		spdk    []string
		errCode codes.Code
		errMsg  string
	}{
		"invalid fieldmask": {
			mask: &fieldmaskpb.FieldMask{Paths: []string{"*", "author"}},
			in: &pb.NvmeNamespace{
				Name: testNamespaceName,
				Spec: testNamespace.Spec,
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("invalid field path: %s", "'*' must not be used with other paths"),
		},
		"unimplemented method": {
			mask: nil,
			in: &pb.NvmeNamespace{
				Name: testNamespaceName,
				Spec: testNamespace.Spec,
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.Unimplemented,
			errMsg:  fmt.Sprintf("%v method is not implemented", "UpdateNvmeNamespace"),
		},
		"valid request with unknown key": {
			mask: nil,
			in: &pb.NvmeNamespace{
				Name: frontend.ResourceIDToNamespaceName(testSubsystemID, "unknown-namespace-id"),
				Spec: testNamespace.Spec,
			},
			out:     nil,
			spdk:    []string{},
			errCode: codes.NotFound,
			errMsg:  fmt.Sprintf("unable to find key %v", frontend.ResourceIDToNamespaceName(testSubsystemID, "unknown-namespace-id")),
		},
		"malformed name": {
			mask: nil,
			in: &pb.NvmeNamespace{
				Name: "-ABC-DEF",
				Spec: testNamespace.Spec,
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

			testEnv.opiSpdkServer.Subsystems[testSubsystemName] = utils.ProtoClone(&testSubsystemWithStatus)
			testEnv.opiSpdkServer.Controllers[testControllerName] = utils.ProtoClone(&testControllerWithStatus)
			testEnv.opiSpdkServer.Namespaces[testNamespaceName] = utils.ProtoClone(&testNamespaceWithStatus)
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
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
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
			in:      testSubsystemName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status":1}}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("Could not list NS: %v", testSubsystemName),
			size:    0,
			token:   "",
		},
		"valid request with invalid marshal SPDK response": {
			in:      testSubsystemName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_subsys_get_ns_list: %v", "json: cannot unmarshal array into Go value of type models.MrvlNvmSubsysGetNsListResult"),
			size:    0,
			token:   "",
		},
		"valid request with empty SPDK response": {
			in:      testSubsystemName,
			out:     nil,
			spdk:    []string{""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_subsys_get_ns_list: %v", "EOF"),
			size:    0,
			token:   "",
		},
		"valid request with ID mismatch SPDK response": {
			in:      testSubsystemName,
			out:     nil,
			spdk:    []string{`{"id":0,"error":{"code":0,"message":""},"result":{"status":1}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_subsys_get_ns_list: %v", "json response ID mismatch"),
			size:    0,
			token:   "",
		},
		"valid request with error code from SPDK response": {
			in:      testSubsystemName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_subsys_get_ns_list: %v", "json response error: myopierr"),
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
			out: []*pb.NvmeNamespace{
				{
					Spec: &pb.NvmeNamespaceSpec{
						HostNsid: 11,
					},
				},
			},
			spdk:    []string{`{"jsonrpc":"2.0","id":%d,"result":{"status":0,"ns_list":[{"ns_instance_id":11,"bdev":"bdev01","ctrlr_id_list":[{"ctrlr_id":1}]},{"ns_instance_id":12,"bdev":"bdev02","ctrlr_id_list":[]},{"ns_instance_id":13,"bdev":"bdev03","ctrlr_id_list":[]}]}}`},
			errCode: codes.OK,
			errMsg:  "",
			size:    1,
			token:   "",
		},
		"pagination overflow": {
			in: testSubsystemName,
			out: []*pb.NvmeNamespace{
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
			spdk:    []string{`{"jsonrpc":"2.0","id":%d,"result":{"status":0,"ns_list":[{"ns_instance_id":11,"bdev":"bdev01","ctrlr_id_list":[{"ctrlr_id":1}]},{"ns_instance_id":12,"bdev":"bdev02","ctrlr_id_list":[]},{"ns_instance_id":13,"bdev":"bdev03","ctrlr_id_list":[]}]}}`},
			errCode: codes.OK,
			errMsg:  "",
			size:    1000,
			token:   "",
		},
		"pagination offset": {
			in: testSubsystemName,
			out: []*pb.NvmeNamespace{
				{
					Spec: &pb.NvmeNamespaceSpec{
						HostNsid: 12,
					},
				},
			},
			spdk:    []string{`{"jsonrpc":"2.0","id":%d,"result":{"status":0,"ns_list":[{"ns_instance_id":11,"bdev":"bdev01","ctrlr_id_list":[{"ctrlr_id":1}]},{"ns_instance_id":12,"bdev":"bdev02","ctrlr_id_list":[]},{"ns_instance_id":13,"bdev":"bdev03","ctrlr_id_list":[]}]}}`},
			errCode: codes.OK,
			errMsg:  "",
			size:    1,
			token:   "existing-pagination-token",
		},
		"valid request with valid SPDK response": {
			in: testSubsystemName,
			out: []*pb.NvmeNamespace{
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
			spdk:    []string{`{"jsonrpc":"2.0","id":%d,"result":{"status":0,"ns_list":[{"ns_instance_id":11,"bdev":"bdev01","ctrlr_id_list":[{"ctrlr_id":1}]},{"ns_instance_id":12,"bdev":"bdev02","ctrlr_id_list":[]},{"ns_instance_id":13,"bdev":"bdev03","ctrlr_id_list":[]}]}}`},
			errCode: codes.OK,
			errMsg:  "",
			size:    0,
			token:   "",
		},
		"valid request with unknown key": {
			in:      frontend.ResourceIDToNamespaceName(testSubsystemID, "unknown-namespace-id"),
			out:     nil,
			spdk:    []string{},
			errCode: codes.NotFound,
			errMsg:  fmt.Sprintf("unable to find key %v", frontend.ResourceIDToNamespaceName(testSubsystemID, "unknown-namespace-id")),
			size:    0,
			token:   "",
		},
		"no required field": {
			in:      "",
			out:     []*pb.NvmeNamespace{},
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

			testEnv.opiSpdkServer.Subsystems[testSubsystemName] = utils.ProtoClone(&testSubsystemWithStatus)
			testEnv.opiSpdkServer.Controllers[testControllerName] = utils.ProtoClone(&testControllerWithStatus)
			testEnv.opiSpdkServer.Namespaces[testNamespaceName] = utils.ProtoClone(&testNamespaceWithStatus)
			testEnv.opiSpdkServer.Pagination["existing-pagination-token"] = 1

			request := &pb.ListNvmeNamespacesRequest{Parent: tt.in, PageSize: tt.size, PageToken: tt.token}
			response, err := testEnv.client.ListNvmeNamespaces(testEnv.ctx, request)

			if !utils.EqualProtoSlices(response.GetNvmeNamespaces(), tt.out) {
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
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		in      string
		out     *pb.NvmeNamespace
		spdk    []string
		errCode codes.Code
		errMsg  string
	}{
		"valid request with invalid SPDK response": {
			in:      testNamespaceName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status":1}}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("Could not get NS: %v", testNamespaceName),
		},
		"valid request with invalid marshal SPDK response": {
			in:      testNamespaceName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_ns_get_info: %v", "json: cannot unmarshal array into Go value of type models.MrvlNvmGetNsInfoResult"),
		},
		"valid request with empty SPDK response": {
			in:      testNamespaceName,
			out:     nil,
			spdk:    []string{""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_ns_get_info: %v", "EOF"),
		},
		"valid request with ID mismatch SPDK response": {
			in:      testNamespaceName,
			out:     nil,
			spdk:    []string{`{"id":0,"error":{"code":0,"message":""},"result":{"status":1}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_ns_get_info: %v", "json response ID mismatch"),
		},
		"valid request with error code from SPDK response": {
			in:      testNamespaceName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_ns_get_info: %v", "json response error: myopierr"),
		},
		"valid request with valid SPDK response": {
			in: testNamespaceName,
			out: &pb.NvmeNamespace{
				Name: testNamespaceName,
				Spec: &pb.NvmeNamespaceSpec{
					Nguid: "0x25f9cbc45d0f976fb9c1a14ff5aed4b0",
				},
				Status: &pb.NvmeNamespaceStatus{
					PciState:     2,
					PciOperState: 1,
				},
			},
			spdk:    []string{`{"jsonrpc":"2.0","id":%d,"result":{"status":0,"nguid":"0x25f9cbc45d0f976fb9c1a14ff5aed4b0","eui64":"0xa7632f80702e4242","uuid":"0xb35633240b77073b8b4ebda571120dfb","nmic":1,"bdev":"bdev01","num_ctrlrs":1,"ctrlr_id_list":[{"ctrlr_id":1}]}}`},
			errCode: codes.OK,
			errMsg:  "",
		},
		"valid request with unknown key": {
			in:      "unknown-namespace-id",
			out:     nil,
			spdk:    []string{},
			errCode: codes.NotFound,
			errMsg:  fmt.Sprintf("unable to find key %v", "unknown-namespace-id"),
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

			testEnv.opiSpdkServer.Subsystems[testSubsystemName] = utils.ProtoClone(&testSubsystemWithStatus)
			testEnv.opiSpdkServer.Controllers[testControllerName] = utils.ProtoClone(&testControllerWithStatus)
			testEnv.opiSpdkServer.Namespaces[testNamespaceName] = utils.ProtoClone(&testNamespaceWithStatus)

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

func TestFrontEnd_StatsNvmeNamespace(t *testing.T) {
	t.Cleanup(checkGlobalTestProtoObjectsNotChanged(t, t.Name()))
	tests := map[string]struct {
		in      string
		out     *pb.VolumeStats
		spdk    []string
		errCode codes.Code
		errMsg  string
	}{
		"valid request with invalid SPDK response": {
			in:      testNamespaceName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			errCode: codes.InvalidArgument,
			errMsg:  fmt.Sprintf("Could not stats NS: %v", testNamespaceName),
		},
		"valid request with invalid marshal SPDK response": {
			in:      testNamespaceName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_get_ns_stats: %v", "json: cannot unmarshal array into Go value of type models.MrvlNvmGetNsStatsResult"),
		},
		"valid request with empty SPDK response": {
			in:      testNamespaceName,
			out:     nil,
			spdk:    []string{""},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_get_ns_stats: %v", "EOF"),
		},
		"valid request with ID mismatch SPDK response": {
			in:      testNamespaceName,
			out:     nil,
			spdk:    []string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_get_ns_stats: %v", "json response ID mismatch"),
		},
		"valid request with error code from SPDK response": {
			in:      testNamespaceName,
			out:     nil,
			spdk:    []string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			errCode: codes.Unknown,
			errMsg:  fmt.Sprintf("mrvl_nvm_get_ns_stats: %v", "json response error: myopierr"),
		},
		"valid request with valid SPDK response": {
			in: testNamespaceName,
			out: &pb.VolumeStats{
				ReadBytesCount:    2,
				ReadOpsCount:      1,
				WriteBytesCount:   4,
				WriteOpsCount:     3,
				ReadLatencyTicks:  6,
				WriteLatencyTicks: 7,
			},
			spdk:    []string{`{"jsonrpc":"2.0","id":%d,"result":{"status":0,"num_read_cmds":1,"num_read_bytes":2,"num_write_cmds":3,"num_write_bytes":4,"num_errors":5,"total_read_latency_in_us":6,"total_write_latency_in_us":7,"Stats_time_window_in_us":8}}`},
			errCode: codes.OK,
			errMsg:  "",
		},
		"valid request with unknown key": {
			in:      "unknown-namespace-id",
			out:     nil,
			spdk:    []string{},
			errCode: codes.NotFound,
			errMsg:  fmt.Sprintf("unable to find key %v", "unknown-namespace-id"),
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

			testEnv.opiSpdkServer.Subsystems[testSubsystemName] = utils.ProtoClone(&testSubsystemWithStatus)
			testEnv.opiSpdkServer.Controllers[testControllerName] = utils.ProtoClone(&testControllerWithStatus)
			testEnv.opiSpdkServer.Namespaces[testNamespaceName] = utils.ProtoClone(&testNamespaceWithStatus)

			request := &pb.StatsNvmeNamespaceRequest{Name: tt.in}
			response, err := testEnv.client.StatsNvmeNamespace(testEnv.ctx, request)

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
