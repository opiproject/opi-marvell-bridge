// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022-2023 Dell Inc, or its subsidiaries.
// Copyright (C) 2022 Marvell International Ltd.
// Copyright (C) 2023 Intel Corporation

// Package frontend implememnts the FrontEnd APIs (host facing) of the storage Server
package frontend

import (
	"context"
	"fmt"
	"log"
	"path"
	"sort"
	"strconv"

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-marvell-bridge/pkg/models"
	"github.com/opiproject/opi-spdk-bridge/pkg/utils"

	"github.com/google/uuid"
	"go.einride.tech/aip/fieldbehavior"
	"go.einride.tech/aip/fieldmask"
	"go.einride.tech/aip/resourceid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

func sortNvmeNamespaces(namespaces []*pb.NvmeNamespace) {
	sort.Slice(namespaces, func(i int, j int) bool {
		return namespaces[i].Spec.HostNsid < namespaces[j].Spec.HostNsid
	})
}

// CreateNvmeNamespace creates an Nvme namespace
func (s *Server) CreateNvmeNamespace(ctx context.Context, in *pb.CreateNvmeNamespaceRequest) (*pb.NvmeNamespace, error) {
	// check input correctness
	if err := s.validateCreateNvmeNamespaceRequest(in); err != nil {
		return nil, err
	}
	// see https://google.aip.dev/133#user-specified-ids
	resourceID := resourceid.NewSystemGenerated()
	if in.NvmeNamespaceId != "" {
		log.Printf("client provided the ID of a resource %v, ignoring the name field %v", in.NvmeNamespaceId, in.NvmeNamespace.Name)
		resourceID = in.NvmeNamespaceId
	}
	in.NvmeNamespace.Name = utils.ResourceIDToNamespaceName(
		utils.GetSubsystemIDFromNvmeName(in.Parent), resourceID,
	)
	// idempotent API when called with same key, should return same object
	namespace, ok := s.Namespaces[in.NvmeNamespace.Name]
	if ok {
		log.Printf("Already existing NvmeNamespace with id %v", in.NvmeNamespace.Name)
		return namespace, nil
	}
	// not found, so create a new one
	subsys, ok := s.Subsystems[in.Parent]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Parent)
		return nil, err
	}
	// TODO: do lookup through VolumeId key instead of using it's value
	params := models.MrvlNvmSubsysAllocNsParams{
		Subnqn:      subsys.Spec.Nqn,
		Nguid:       in.NvmeNamespace.Spec.Nguid,
		Eui64:       strconv.FormatInt(in.NvmeNamespace.Spec.Eui64, 10),
		UUID:        in.NvmeNamespace.Spec.Uuid.Value,
		ShareEnable: 1,
		Bdev:        in.NvmeNamespace.Spec.VolumeNameRef,
	}
	var result models.MrvlNvmSubsysAllocNsResult
	err := s.rpc.Call(ctx, "mrvl_nvm_subsys_alloc_ns", &params, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		msg := fmt.Sprintf("Could not create NS: %s", in.NvmeNamespace.Name)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	// Now, attach this new NS to ALL controllers
	for _, c := range s.Controllers {
		if utils.GetSubsystemIDFromNvmeName(c.Name) != utils.GetSubsystemIDFromNvmeName(in.Parent) {
			continue
		}
		params := models.MrvlNvmCtrlrAttachNsParams{
			Subnqn:       subsys.Spec.Nqn,
			CtrlrID:      int(*c.Spec.NvmeControllerId),
			NsInstanceID: int(in.NvmeNamespace.Spec.HostNsid),
		}
		var result models.MrvlNvmCtrlrAttachNsResult
		err := s.rpc.Call(ctx, "mrvl_nvm_ctrlr_attach_ns", &params, &result)
		if err != nil {
			return nil, err
		}
		if result.Status != 0 {
			msg := fmt.Sprintf("Could not attach NS: %s", in.NvmeNamespace.Name)
			return nil, status.Errorf(codes.InvalidArgument, msg)
		}
	}
	response := utils.ProtoClone(in.NvmeNamespace)
	response.Status = &pb.NvmeNamespaceStatus{
		State:     pb.NvmeNamespaceStatus_STATE_ENABLED,
		OperState: pb.NvmeNamespaceStatus_OPER_STATE_ONLINE,
	}
	s.Namespaces[in.NvmeNamespace.Name] = response
	return response, nil
}

// DeleteNvmeNamespace deletes an Nvme namespace
func (s *Server) DeleteNvmeNamespace(ctx context.Context, in *pb.DeleteNvmeNamespaceRequest) (*emptypb.Empty, error) {
	// check input correctness
	if err := s.validateDeleteNvmeNamespaceRequest(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	namespace, ok := s.Namespaces[in.Name]
	if !ok {
		if in.AllowMissing {
			return &emptypb.Empty{}, nil
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		return nil, err
	}
	subsysName := utils.ResourceIDToSubsystemName(
		utils.GetSubsystemIDFromNvmeName(in.Name),
	)
	subsys, ok := s.Subsystems[subsysName]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find subsystem %s", subsysName)
		return nil, err
	}
	// First, detach this NS from ALL controllers
	for _, c := range s.Controllers {
		if utils.GetSubsystemIDFromNvmeName(c.Name) != utils.GetSubsystemIDFromNvmeName(in.Name) {
			continue
		}
		params := models.MrvlNvmCtrlrDetachNsParams{
			Subnqn:       subsys.Spec.Nqn,
			CtrlrID:      int(*c.Spec.NvmeControllerId),
			NsInstanceID: int(namespace.Spec.HostNsid),
		}
		var result models.MrvlNvmCtrlrDetachNsResult
		err := s.rpc.Call(ctx, "mrvl_nvm_ctrlr_detach_ns", &params, &result)
		if err != nil {
			return nil, err
		}
		log.Printf("Received from SPDK: %v", result)
		if result.Status != 0 {
			msg := fmt.Sprintf("Could not detach NS: %s", in.Name)
			return nil, status.Errorf(codes.InvalidArgument, msg)
		}
	}
	params := models.MrvlNvmSubsysUnallocNsParams{
		Subnqn:       subsys.Spec.Nqn,
		NsInstanceID: int(namespace.Spec.HostNsid),
	}
	var result models.MrvlNvmSubsysUnallocNsResult
	err := s.rpc.Call(ctx, "mrvl_nvm_subsys_unalloc_ns", &params, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		msg := fmt.Sprintf("Could not delete NS: %s", in.Name)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	delete(s.Namespaces, namespace.Name)
	return &emptypb.Empty{}, nil
}

// UpdateNvmeNamespace updates an Nvme namespace
func (s *Server) UpdateNvmeNamespace(_ context.Context, in *pb.UpdateNvmeNamespaceRequest) (*pb.NvmeNamespace, error) {
	// check input correctness
	if err := s.validateUpdateNvmeNamespaceRequest(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	volume, ok := s.Namespaces[in.NvmeNamespace.Name]
	if !ok {
		if in.AllowMissing {
			log.Printf("TODO: in case of AllowMissing, create a new resource, don;t return error")
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.NvmeNamespace.Name)
		return nil, err
	}
	resourceID := path.Base(volume.Name)
	// update_mask = 2
	if err := fieldmask.Validate(in.UpdateMask, in.NvmeNamespace); err != nil {
		return nil, err
	}
	log.Printf("TODO: use resourceID=%v", resourceID)
	return nil, status.Errorf(codes.Unimplemented, "UpdateNvmeNamespace method is not implemented")
}

// ListNvmeNamespaces lists Nvme namespaces
func (s *Server) ListNvmeNamespaces(ctx context.Context, in *pb.ListNvmeNamespacesRequest) (*pb.ListNvmeNamespacesResponse, error) {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	size, offset, perr := utils.ExtractPagination(in.PageSize, in.PageToken, s.Pagination)
	if perr != nil {
		return nil, perr
	}
	subsys, ok := s.Subsystems[in.Parent]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Parent)
		return nil, err
	}
	params := models.MrvlNvmSubsysGetNsListParams{
		Subnqn: subsys.Spec.Nqn,
	}
	var result models.MrvlNvmSubsysGetNsListResult
	err := s.rpc.Call(ctx, "mrvl_nvm_subsys_get_ns_list", &params, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		msg := fmt.Sprintf("Could not list NS: %s", in.Parent)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	token, hasMoreElements := "", false
	log.Printf("Limiting result len(%d) to [%d:%d]", len(result.NsList), offset, size)
	result.NsList, hasMoreElements = utils.LimitPagination(result.NsList, offset, size)
	if hasMoreElements {
		token = uuid.New().String()
		s.Pagination[token] = offset + size
	}
	Blobarray := make([]*pb.NvmeNamespace, len(result.NsList))
	for i := range result.NsList {
		r := &result.NsList[i]
		Blobarray[i] = &pb.NvmeNamespace{Spec: &pb.NvmeNamespaceSpec{HostNsid: int32(r.NsInstanceID)}}
	}
	sortNvmeNamespaces(Blobarray)
	return &pb.ListNvmeNamespacesResponse{NvmeNamespaces: Blobarray}, nil
}

// GetNvmeNamespace gets an Nvme namespace
func (s *Server) GetNvmeNamespace(ctx context.Context, in *pb.GetNvmeNamespaceRequest) (*pb.NvmeNamespace, error) {
	// check input correctness
	if err := s.validateGetNvmeNamespaceRequest(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	namespace, ok := s.Namespaces[in.Name]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		return nil, err
	}
	log.Printf("namespace: %v", namespace)
	subsysName := utils.ResourceIDToSubsystemName(
		utils.GetSubsystemIDFromNvmeName(in.Name),
	)
	subsys, ok := s.Subsystems[subsysName]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", subsysName)
		return nil, err
	}

	params := models.MrvlNvmGetNsInfoParams{
		SubNqn:       subsys.Spec.Nqn,
		NsInstanceID: int(namespace.Spec.HostNsid),
	}
	var result models.MrvlNvmGetNsInfoResult
	err := s.rpc.Call(ctx, "mrvl_nvm_ns_get_info", &params, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		msg := fmt.Sprintf("Could not get NS: %s", in.Name)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	return &pb.NvmeNamespace{Name: in.Name,
		Spec: &pb.NvmeNamespaceSpec{
			Nguid: result.Nguid,
		},
		Status: &pb.NvmeNamespaceStatus{
			State:     pb.NvmeNamespaceStatus_STATE_ENABLED,
			OperState: pb.NvmeNamespaceStatus_OPER_STATE_ONLINE,
		},
	}, nil
}

// StatsNvmeNamespace gets an Nvme namespace stats
func (s *Server) StatsNvmeNamespace(ctx context.Context, in *pb.StatsNvmeNamespaceRequest) (*pb.StatsNvmeNamespaceResponse, error) {
	// check input correctness
	if err := s.validateStatsNvmeNamespaceRequest(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	namespace, ok := s.Namespaces[in.Name]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		return nil, err
	}
	subsysName := utils.ResourceIDToSubsystemName(
		utils.GetSubsystemIDFromNvmeName(in.Name),
	)
	subsys, ok := s.Subsystems[subsysName]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", subsysName)
		return nil, err
	}

	params := models.MrvlNvmGetNsStatsParams{
		SubNqn:       subsys.Spec.Nqn,
		NsInstanceID: int(namespace.Spec.HostNsid),
	}
	var result models.MrvlNvmGetNsStatsResult
	err := s.rpc.Call(ctx, "mrvl_nvm_get_ns_stats", &params, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		msg := fmt.Sprintf("Could not stats NS: %s", in.Name)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	return &pb.StatsNvmeNamespaceResponse{Stats: &pb.VolumeStats{
		ReadBytesCount:    int32(result.NumReadBytes),
		ReadOpsCount:      int32(result.NumReadCmds),
		WriteBytesCount:   int32(result.NumWriteBytes),
		WriteOpsCount:     int32(result.NumWriteCmds),
		ReadLatencyTicks:  int32(result.TotalReadLatencyInUs),
		WriteLatencyTicks: int32(result.TotalWriteLatencyInUs),
	}}, nil
}
