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

func TestFrontEnd_CreateNvmeSubsystem(t *testing.T) {
	spec := &pb.NvmeSubsystemSpec{
		Nqn:          "nqn.2022-09.io.spdk:opi3",
		SerialNumber: "OpiSerialNumber",
		ModelNumber:  "OpiModelNumber",
	}
	tests := map[string]struct {
		id      string
		in      *pb.NvmeSubsystem
		out     *pb.NvmeSubsystem
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
		exist   bool
	}{
		"illegal resource_id": {
			"CapitalLettersNotAllowed",
			&pb.NvmeSubsystem{
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
			testSubsystemID,
			&pb.NvmeSubsystem{
				Name: testSubsystemName,
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
			testSubsystemID,
			&pb.NvmeSubsystem{
				Name: testSubsystemName,
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
			testSubsystemID,
			&pb.NvmeSubsystem{
				Name: testSubsystemName,
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
			testSubsystemID,
			&pb.NvmeSubsystem{
				Name: testSubsystemName,
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
			testSubsystemID,
			&pb.NvmeSubsystem{
				Name: testSubsystemName,
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
			testSubsystemID,
			&pb.NvmeSubsystem{
				Name: testSubsystemName,
				Spec: spec,
			},
			&pb.NvmeSubsystem{
				Name: testSubsystemName,
				Spec: spec,
				Status: &pb.NvmeSubsystemStatus{
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
			testSubsystemID,
			&pb.NvmeSubsystem{
				Name: testSubsystemName,
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

			testEnv.opiSpdkServer.Controllers[testControllerName] = &testController
			testEnv.opiSpdkServer.Namespaces[testNamespaceName] = &testNamespace
			if tt.exist {
				testEnv.opiSpdkServer.Subsystems[testSubsystemName] = &testSubsystem
				// testEnv.opiSpdkServer.Subsystems[testSubsystemID].Spec.Id = &pc.ObjectKey{Value: testSubsystemID}
			}
			if tt.out != nil {
				tt.out.Name = testSubsystemName
			}

			request := &pb.CreateNvmeSubsystemRequest{NvmeSubsystem: tt.in, NvmeSubsystemId: tt.id}
			response, err := testEnv.client.CreateNvmeSubsystem(testEnv.ctx, request)
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

func TestFrontEnd_DeleteNvmeSubsystem(t *testing.T) {
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
			testSubsystemName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not delete NQN: %v", "nqn.2022-09.io.spdk:opi3"),
			true,
			false,
		},
		"valid request with empty SPDK response": {
			testSubsystemName,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_delete_subsystem: %v", "EOF"),
			true,
			false,
		},
		"valid request with ID mismatch SPDK response": {
			testSubsystemName,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_delete_subsystem: %v", "json response ID mismatch"),
			true,
			false,
		},
		"valid request with error code from SPDK response": {
			testSubsystemName,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_delete_subsystem: %v", "json response error: myopierr"),
			true,
			false,
		},
		"valid request with valid SPDK response": {
			testSubsystemName,
			&emptypb.Empty{},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0}}`},
			codes.OK,
			"",
			true,
			false,
		},
		"valid request with unknown key": {
			server.ResourceIDToVolumeName("unknown-subsystem-id"),
			nil,
			[]string{""},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", server.ResourceIDToVolumeName("unknown-subsystem-id")),
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

			request := &pb.DeleteNvmeSubsystemRequest{Name: tt.in, AllowMissing: tt.missing}
			response, err := testEnv.client.DeleteNvmeSubsystem(testEnv.ctx, request)
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

func TestFrontEnd_UpdateNvmeSubsystem(t *testing.T) {
	tests := map[string]struct {
		mask    *fieldmaskpb.FieldMask
		in      *pb.NvmeSubsystem
		out     *pb.NvmeSubsystem
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		"invalid fieldmask": {
			&fieldmaskpb.FieldMask{Paths: []string{"*", "author"}},
			&pb.NvmeSubsystem{
				Name: testSubsystemName,
			},
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("invalid field path: %s", "'*' must not be used with other paths"),
			false,
		},
		"unimplemented method": {
			nil,
			&pb.NvmeSubsystem{
				Name: testSubsystemName,
			},
			nil,
			[]string{""},
			codes.Unimplemented,
			fmt.Sprintf("%v method is not implemented", "UpdateNvmeSubsystem"),
			false,
		},
		"valid request with unknown key": {
			nil,
			&pb.NvmeSubsystem{
				Name: server.ResourceIDToVolumeName("unknown-id"),
				Spec: &pb.NvmeSubsystemSpec{
					Nqn: "nqn.2022-09.io.spdk:opi3",
				},
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

			request := &pb.UpdateNvmeSubsystemRequest{NvmeSubsystem: tt.in, UpdateMask: tt.mask}
			response, err := testEnv.client.UpdateNvmeSubsystem(testEnv.ctx, request)
			if response != nil {
				t.Error("response: expected", tt.out, "received", response)
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

func TestFrontEnd_ListNvmeSubsystem(t *testing.T) {
	tests := map[string]struct {
		out     []*pb.NvmeSubsystem
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
			[]*pb.NvmeSubsystem{
				{Spec: &pb.NvmeSubsystemSpec{Nqn: "nqn.2022-09.io.spdk:opi1"}},
			},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0, "subsys_list": [{"subnqn": "nqn.2022-09.io.spdk:opi1"},{"subnqn": "nqn.2022-09.io.spdk:opi2"},{"subnqn": "nqn.2022-09.io.spdk:opi3"}]}}`},
			codes.OK,
			"",
			true,
			1,
			"",
		},
		"pagination overflow": {
			[]*pb.NvmeSubsystem{
				{Spec: &pb.NvmeSubsystemSpec{Nqn: "nqn.2022-09.io.spdk:opi1"}},
				{Spec: &pb.NvmeSubsystemSpec{Nqn: "nqn.2022-09.io.spdk:opi2"}},
				{Spec: &pb.NvmeSubsystemSpec{Nqn: "nqn.2022-09.io.spdk:opi3"}},
			},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0, "subsys_list": [{"subnqn": "nqn.2022-09.io.spdk:opi1"},{"subnqn": "nqn.2022-09.io.spdk:opi2"},{"subnqn": "nqn.2022-09.io.spdk:opi3"}]}}`},
			codes.OK,
			"",
			true,
			1000,
			"",
		},
		"pagination offset": {
			[]*pb.NvmeSubsystem{
				{Spec: &pb.NvmeSubsystemSpec{Nqn: "nqn.2022-09.io.spdk:opi2"}},
			},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0, "subsys_list": [{"subnqn": "nqn.2022-09.io.spdk:opi1"},{"subnqn": "nqn.2022-09.io.spdk:opi2"},{"subnqn": "nqn.2022-09.io.spdk:opi3"}]}}`},
			codes.OK,
			"",
			true,
			1,
			"existing-pagination-token",
		},
		"valid request with valid SPDK response": {
			[]*pb.NvmeSubsystem{
				{Spec: &pb.NvmeSubsystemSpec{Nqn: "nqn.2022-09.io.spdk:opi1"}},
				{Spec: &pb.NvmeSubsystemSpec{Nqn: "nqn.2022-09.io.spdk:opi2"}},
				{Spec: &pb.NvmeSubsystemSpec{Nqn: "nqn.2022-09.io.spdk:opi3"}},
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

			testEnv.opiSpdkServer.Subsystems[testSubsystemName] = &testSubsystem
			testEnv.opiSpdkServer.Controllers[testControllerName] = &testController
			testEnv.opiSpdkServer.Namespaces[testNamespaceName] = &testNamespace
			testEnv.opiSpdkServer.Pagination["existing-pagination-token"] = 1

			request := &pb.ListNvmeSubsystemsRequest{PageSize: tt.size, PageToken: tt.token}
			response, err := testEnv.client.ListNvmeSubsystems(testEnv.ctx, request)
			if response != nil {
				if !reflect.DeepEqual(response.NvmeSubsystems, tt.out) {
					t.Error("response: expected", tt.out, "received", response.NvmeSubsystems)
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

func TestFrontEnd_GetNvmeSubsystem(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *pb.NvmeSubsystem
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		"valid request with invalid SPDK response": {
			testSubsystemName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not list NQN: %v", "nqn.2022-09.io.spdk:opi3"),
			true,
		},
		"valid request with empty SPDK response": {
			testSubsystemName,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_get_subsys_list: %v", "EOF"),
			true,
		},
		"valid request with ID mismatch SPDK response": {
			testSubsystemName,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_get_subsys_list: %v", "json response ID mismatch"),
			true,
		},
		"valid request with error code from SPDK response": {
			testSubsystemName,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_get_subsys_list: %v", "json response error: myopierr"),
			true,
		},
		"valid request with valid SPDK response": {
			testSubsystemName,
			&pb.NvmeSubsystem{
				Spec:   &pb.NvmeSubsystemSpec{Nqn: "nqn.2022-09.io.spdk:opi3"},
				Status: &pb.NvmeSubsystemStatus{FirmwareRevision: "TBD"},
			},
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 0, "subsys_list": [{"subnqn": "nqn.2022-09.io.spdk:opi1"},{"subnqn": "nqn.2022-09.io.spdk:opi2"},{"subnqn": "nqn.2022-09.io.spdk:opi3"}]}}`},
			codes.OK,
			"",
			true,
		},
		"valid request with unknown key": {
			server.ResourceIDToVolumeName("unknown-subsystem-id"),
			nil,
			[]string{""},
			codes.NotFound,
			fmt.Sprintf("unable to find key %v", server.ResourceIDToVolumeName("unknown-subsystem-id")),
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

			request := &pb.GetNvmeSubsystemRequest{Name: tt.in}
			response, err := testEnv.client.GetNvmeSubsystem(testEnv.ctx, request)
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

func TestFrontEnd_NvmeSubsystemStats(t *testing.T) {
	tests := map[string]struct {
		in      string
		out     *pb.VolumeStats
		spdk    []string
		errCode codes.Code
		errMsg  string
		start   bool
	}{
		"valid request with invalid SPDK response": {
			testSubsystemName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.InvalidArgument,
			fmt.Sprintf("Could not stats NQN: %v", "nqn.2022-09.io.spdk:opi3"),
			true,
		},
		"valid request with invalid marshal SPDK response": {
			testSubsystemName,
			nil,
			[]string{`{"id":%d,"error":{"code":0,"message":""},"result":[]}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_get_info: %v", "json: cannot unmarshal array into Go value of type models.MrvlNvmGetSubsysInfoResult"),
			true,
		},
		"valid request with empty SPDK response": {
			testSubsystemName,
			nil,
			[]string{""},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_get_info: %v", "EOF"),
			true,
		},
		"valid request with ID mismatch SPDK response": {
			testSubsystemName,
			nil,
			[]string{`{"id":0,"error":{"code":0,"message":""},"result":{"status": 1}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_get_info: %v", "json response ID mismatch"),
			true,
		},
		"valid request with error code from SPDK response": {
			testSubsystemName,
			nil,
			[]string{`{"id":%d,"error":{"code":1,"message":"myopierr"}}`},
			codes.Unknown,
			fmt.Sprintf("mrvl_nvm_subsys_get_info: %v", "json response error: myopierr"),
			true,
		},
		"valid request with valid SPDK response": {
			testSubsystemName,
			&pb.VolumeStats{
				ReadOpsCount:  -1,
				WriteOpsCount: -1,
			},
			[]string{`{"jsonrpc":"2.0","id":%d,"result":{"status":0,"subsys_list":[{"subnqn":"nqn.2014-08.org.Nvmexpress.discovery","mn":"OCTEON NVME 0.0.1","sn":"OCTNVME0000000000002","max_namespaces":16,"min_ctrlr_id":1,"max_ctrlr_id":8,"num_ns":2,"num_total_ctrlr":2,"num_active_ctrlr":2,"ns_list":[{"ns_instance_id":1,"bdev":"bdev01","ctrlr_id_list":[{"ctrlr_id":1},{"ctrlr_id":2}]},{"ns_instance_id":1,"bdev":"bdev02","ctrlr_id_list":[{"ctrlr_id":3}]}]}]}}`},
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

			testEnv.opiSpdkServer.Subsystems[testSubsystemName] = &testSubsystem
			testEnv.opiSpdkServer.Controllers[testControllerName] = &testController
			testEnv.opiSpdkServer.Namespaces[testNamespaceName] = &testNamespace

			request := &pb.NvmeSubsystemStatsRequest{SubsystemId: &pc.ObjectKey{Value: tt.in}}
			response, err := testEnv.client.NvmeSubsystemStats(testEnv.ctx, request)
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
