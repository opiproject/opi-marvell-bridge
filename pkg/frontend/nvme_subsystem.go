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

	"github.com/opiproject/gospdk/spdk"
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

func sortNvmeSubsystems(subsystems []*pb.NvmeSubsystem) {
	sort.Slice(subsystems, func(i int, j int) bool {
		return subsystems[i].Spec.Nqn < subsystems[j].Spec.Nqn
	})
}

// CreateNvmeSubsystem creates an Nvme Subsystem
func (s *Server) CreateNvmeSubsystem(ctx context.Context, in *pb.CreateNvmeSubsystemRequest) (*pb.NvmeSubsystem, error) {
	// check input correctness
	if err := s.validateCreateNvmeSubsystemRequest(in); err != nil {
		return nil, err
	}
	// see https://google.aip.dev/133#user-specified-ids
	resourceID := resourceid.NewSystemGenerated()
	if in.NvmeSubsystemId != "" {
		log.Printf("client provided the ID of a resource %v, ignoring the name field %v", in.NvmeSubsystemId, in.NvmeSubsystem.Name)
		resourceID = in.NvmeSubsystemId
	}
	in.NvmeSubsystem.Name = utils.ResourceIDToSubsystemName(resourceID)
	// idempotent API when called with same key, should return same object
	subsys := new(pb.NvmeSubsystem)
	found, err := s.store.Get(in.NvmeSubsystem.Name, subsys)
	if err != nil {
		return nil, err
	}
	if found {
		log.Printf("Already existing NvmeSubsystem with id %v", in.NvmeSubsystem.Name)
		return subsys, nil
	}
	// check if another object exists with same NQN, it is not allowed
	for _, item := range s.Subsystems {
		if in.NvmeSubsystem.Spec.Nqn == item.Spec.Nqn {
			msg := fmt.Sprintf("Could not create NQN: %s since object %s with same NQN already exists", in.NvmeSubsystem.Spec.Nqn, item.Name)
			return nil, status.Errorf(codes.AlreadyExists, msg)
		}
	}
	// not found, so create a new one

	// TODO: fix const values below
	params := models.MrvlNvmCreateSubsystemParams{
		Subnqn:        in.NvmeSubsystem.Spec.Nqn,
		Mn:            in.NvmeSubsystem.Spec.ModelNumber,
		Sn:            in.NvmeSubsystem.Spec.SerialNumber,
		MaxNamespaces: int(in.NvmeSubsystem.Spec.MaxNamespaces),
		MinCtrlrID:    0, // bug in v21.01, should be 0 for now
		MaxCtrlrID:    256,
	}
	var result models.MrvlNvmCreateSubsystemResult
	err = s.rpc.Call(ctx, "mrvl_nvm_create_subsystem", &params, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		msg := fmt.Sprintf("Could not create NQN: %s", in.NvmeSubsystem.Spec.Nqn)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	var ver spdk.GetVersionResult
	err = s.rpc.Call(ctx, "spdk_get_version", nil, &ver)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", ver)
	response := utils.ProtoClone(in.NvmeSubsystem)
	response.Status = &pb.NvmeSubsystemStatus{FirmwareRevision: ver.Version}
	err = s.store.Set(in.NvmeSubsystem.Name, response)
	if err != nil {
		return nil, err
	}
	return response, nil
}

// DeleteNvmeSubsystem deletes an Nvme Subsystem
func (s *Server) DeleteNvmeSubsystem(ctx context.Context, in *pb.DeleteNvmeSubsystemRequest) (*emptypb.Empty, error) {
	// check input correctness
	if err := s.validateDeleteNvmeSubsystemRequest(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	subsys := new(pb.NvmeSubsystem)
	found, err := s.store.Get(in.Name, subsys)
	if err != nil {
		return nil, err
	}
	if !found {
		if in.AllowMissing {
			return &emptypb.Empty{}, nil
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		return nil, err
	}
	params := models.MrvlNvmDeleteSubsystemParams{
		Subnqn: subsys.Spec.Nqn,
	}
	var result models.MrvlNvmDeleteSubsystemResult
	err = s.rpc.Call(ctx, "mrvl_nvm_delete_subsystem", &params, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		msg := fmt.Sprintf("Could not delete NQN: %s", subsys.Spec.Nqn)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	// remove from the Database
	err = s.store.Delete(subsys.Name)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// UpdateNvmeSubsystem updates an Nvme Subsystem
func (s *Server) UpdateNvmeSubsystem(_ context.Context, in *pb.UpdateNvmeSubsystemRequest) (*pb.NvmeSubsystem, error) {
	// check input correctness
	if err := s.validateUpdateNvmeSubsystemRequest(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	subsys := new(pb.NvmeSubsystem)
	found, err := s.store.Get(in.NvmeSubsystem.Name, subsys)
	if err != nil {
		return nil, err
	}
	if !found {
		if in.AllowMissing {
			log.Printf("TODO: in case of AllowMissing, create a new resource, don;t return error")
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.NvmeSubsystem.Name)
		return nil, err
	}
	resourceID := path.Base(subsys.Name)
	// update_mask = 2
	if err := fieldmask.Validate(in.UpdateMask, in.NvmeSubsystem); err != nil {
		return nil, err
	}
	log.Printf("TODO: use resourceID=%v", resourceID)
	return nil, status.Errorf(codes.Unimplemented, "UpdateNvmeSubsystem method is not implemented")
}

// ListNvmeSubsystems lists Nvme Subsystems
func (s *Server) ListNvmeSubsystems(ctx context.Context, in *pb.ListNvmeSubsystemsRequest) (*pb.ListNvmeSubsystemsResponse, error) {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	size, offset, perr := utils.ExtractPagination(in.PageSize, in.PageToken, s.Pagination)
	if perr != nil {
		return nil, perr
	}
	var result models.MrvlNvmGetSubsysListResult
	err := s.rpc.Call(ctx, "mrvl_nvm_get_subsys_list", nil, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		msg := "Could not list subsystems"
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	token, hasMoreElements := "", false
	log.Printf("Limiting result len(%d) to [%d:%d]", len(result.SubsysList), offset, size)
	result.SubsysList, hasMoreElements = utils.LimitPagination(result.SubsysList, offset, size)
	if hasMoreElements {
		token = uuid.New().String()
		s.Pagination[token] = offset + size
	}
	Blobarray := make([]*pb.NvmeSubsystem, len(result.SubsysList))
	for i := range result.SubsysList {
		r := &result.SubsysList[i]
		Blobarray[i] = &pb.NvmeSubsystem{Spec: &pb.NvmeSubsystemSpec{Nqn: r.Subnqn}}
	}
	sortNvmeSubsystems(Blobarray)
	return &pb.ListNvmeSubsystemsResponse{NvmeSubsystems: Blobarray}, nil
}

// GetNvmeSubsystem gets Nvme Subsystems
func (s *Server) GetNvmeSubsystem(ctx context.Context, in *pb.GetNvmeSubsystemRequest) (*pb.NvmeSubsystem, error) {
	// check input correctness
	if err := s.validateGetNvmeSubsystemRequest(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	subsys := new(pb.NvmeSubsystem)
	found, err := s.store.Get(in.Name, subsys)
	if err != nil {
		return nil, err
	}
	if !found {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		return nil, err
	}
	// TODO: replace with MRVL code : mrvl_nvm_subsys_get_info ?
	var result models.MrvlNvmGetSubsysListResult
	err = s.rpc.Call(ctx, "mrvl_nvm_get_subsys_list", nil, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		msg := fmt.Sprintf("Could not list NQN: %s", subsys.Spec.Nqn)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	for i := range result.SubsysList {
		r := &result.SubsysList[i]
		if r.Subnqn == subsys.Spec.Nqn {
			return &pb.NvmeSubsystem{Spec: &pb.NvmeSubsystemSpec{Nqn: r.Subnqn}, Status: &pb.NvmeSubsystemStatus{FirmwareRevision: "TBD"}}, nil
		}
	}
	msg := fmt.Sprintf("Could not find NQN: %s", subsys.Spec.Nqn)
	return nil, status.Errorf(codes.InvalidArgument, msg)
}

// StatsNvmeSubsystem gets Nvme Subsystem stats
func (s *Server) StatsNvmeSubsystem(ctx context.Context, in *pb.StatsNvmeSubsystemRequest) (*pb.StatsNvmeSubsystemResponse, error) {
	// check input correctness
	if err := s.validateStatsNvmeSubsystemRequest(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	subsys := new(pb.NvmeSubsystem)
	found, err := s.store.Get(in.Name, subsys)
	if err != nil {
		return nil, err
	}
	if !found {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		return nil, err
	}
	params := models.MrvlNvmGetSubsysInfoParams{
		Subnqn: subsys.Spec.Nqn,
	}
	var result models.MrvlNvmGetSubsysInfoResult
	err = s.rpc.Call(ctx, "mrvl_nvm_subsys_get_info", &params, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		msg := fmt.Sprintf("Could not stats NQN: %s", subsys.Spec.Nqn)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	return &pb.StatsNvmeSubsystemResponse{Stats: &pb.VolumeStats{ReadOpsCount: -1, WriteOpsCount: -1}}, nil
}
