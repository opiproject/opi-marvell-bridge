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

	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-marvell-bridge/pkg/models"
	"github.com/opiproject/opi-spdk-bridge/pkg/utils"

	"github.com/google/uuid"
	"go.einride.tech/aip/fieldbehavior"
	"go.einride.tech/aip/fieldmask"
	"go.einride.tech/aip/resourceid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/emptypb"
)

const autoCtrlrIDAllocation = -1

func sortNvmeControllers(controllers []*pb.NvmeController) {
	sort.Slice(controllers, func(i int, j int) bool {
		return *controllers[i].Spec.NvmeControllerId < *controllers[j].Spec.NvmeControllerId
	})
}

// CreateNvmeController creates an Nvme controller
func (s *Server) CreateNvmeController(ctx context.Context, in *pb.CreateNvmeControllerRequest) (*pb.NvmeController, error) {
	// check input correctness
	if err := s.validateCreateNvmeControllerRequest(in); err != nil {
		return nil, err
	}
	// see https://google.aip.dev/133#user-specified-ids
	resourceID := resourceid.NewSystemGenerated()
	if in.NvmeControllerId != "" {
		log.Printf("client provided the ID of a resource %v, ignoring the name field %v", in.NvmeControllerId, in.NvmeController.Name)
		resourceID = in.NvmeControllerId
	}
	in.NvmeController.Name = utils.ResourceIDToControllerName(
		utils.GetSubsystemIDFromNvmeName(in.Parent), resourceID,
	)
	// idempotent API when called with same key, should return same object
	controller := new(pb.NvmeController)
	found, err := s.store.Get(in.NvmeController.Name, controller)
	if err != nil {
		return nil, err
	}
	if found {
		log.Printf("Already existing NvmeController with id %v", in.NvmeController.Name)
		return controller, nil
	}
	// not found, so create a new one
	subsys := new(pb.NvmeSubsystem)
	found, err = s.store.Get(in.Parent, subsys)
	if err != nil {
		return nil, err
	}
	if !found {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Parent)
		return nil, err
	}

	ctrlrID := autoCtrlrIDAllocation
	if in.NvmeController.Spec.NvmeControllerId != nil {
		ctrlrID = int(*in.NvmeController.Spec.NvmeControllerId)
	}
	params := models.MrvlNvmSubsysCreateCtrlrParams{
		Subnqn:       subsys.Spec.Nqn,
		PcieDomainID: int(in.GetNvmeController().GetSpec().GetPcieId().GetPortId().GetValue()),
		PfID:         int(in.GetNvmeController().GetSpec().GetPcieId().GetPhysicalFunction().GetValue()),
		VfID:         int(in.GetNvmeController().GetSpec().GetPcieId().GetVirtualFunction().GetValue()),
		CtrlrID:      ctrlrID,
		MaxNsq:       int(in.GetNvmeController().GetSpec().GetMaxNsq()),
		MaxNcq:       int(in.GetNvmeController().GetSpec().GetMaxNcq()),
		Mqes:         int(in.GetNvmeController().GetSpec().GetSqes()),
	}
	var result models.MrvlNvmSubsysCreateCtrlrResult
	err = s.rpc.Call(ctx, "mrvl_nvm_subsys_create_ctrlr", &params, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		msg := fmt.Sprintf("Could not create CTRL: %s", in.NvmeController.Name)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	response := utils.ProtoClone(in.NvmeController)
	response.Spec.NvmeControllerId = proto.Int32(int32(result.CtrlrID))
	response.Status = &pb.NvmeControllerStatus{Active: true}
	err = s.store.Set(in.NvmeController.Name, response)
	if err != nil {
		return nil, err
	}
	return response, nil
}

// DeleteNvmeController deletes an Nvme controller
func (s *Server) DeleteNvmeController(ctx context.Context, in *pb.DeleteNvmeControllerRequest) (*emptypb.Empty, error) {
	// check input correctness
	if err := s.validateDeleteNvmeControllerRequest(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	controller := new(pb.NvmeController)
	found, err := s.store.Get(in.Name, controller)
	if err != nil {
		return nil, err
	}
	if !found {
		if in.AllowMissing {
			return &emptypb.Empty{}, nil
		}
		return nil, fmt.Errorf("error finding controller %s", in.Name)
	}
	subsysName := utils.ResourceIDToSubsystemName(
		utils.GetSubsystemIDFromNvmeName(in.Name),
	)
	// fetch object from the database
	subsys := new(pb.NvmeSubsystem)
	found, err = s.store.Get(subsysName, subsys)
	if err != nil {
		return nil, err
	}
	if !found {
		err := status.Errorf(codes.NotFound, "unable to find key %s", subsysName)
		return nil, err
	}
	// construct command with parameters
	params := models.MrvlNvmSubsysRemoveCtrlrParams{
		Subnqn:  subsys.Spec.Nqn,
		CtrlrID: int(*controller.Spec.NvmeControllerId),
		Force:   1,
	}
	var result models.MrvlNvmSubsysRemoveCtrlrResult
	err = s.rpc.Call(ctx, "mrvl_nvm_subsys_remove_ctrlr", &params, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		msg := fmt.Sprintf("Could not delete CTRL: %s", controller.Name)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	// remove from the Database
	err = s.store.Delete(controller.Name)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// UpdateNvmeController updates an Nvme controller
func (s *Server) UpdateNvmeController(ctx context.Context, in *pb.UpdateNvmeControllerRequest) (*pb.NvmeController, error) {
	// check input correctness
	if err := s.validateUpdateNvmeControllerRequest(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	controller := new(pb.NvmeController)
	found, err := s.store.Get(in.NvmeController.Name, controller)
	if err != nil {
		return nil, err
	}
	if !found {
		if in.AllowMissing {
			log.Printf("TODO: in case of AllowMissing, create a new resource, don;t return error")
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.NvmeController.Name)
		return nil, err
	}
	resourceID := path.Base(controller.Name)
	// update_mask = 2
	if err := fieldmask.Validate(in.UpdateMask, in.NvmeController); err != nil {
		return nil, err
	}
	log.Printf("TODO: use resourceID=%v", resourceID)
	subsysName := utils.ResourceIDToSubsystemName(
		utils.GetSubsystemIDFromNvmeName(in.NvmeController.Name),
	)
	// fetch object from the database
	subsys := new(pb.NvmeSubsystem)
	found, err = s.store.Get(subsysName, subsys)
	if err != nil {
		return nil, err
	}
	if !found {
		err := status.Errorf(codes.NotFound, "unable to find key %s", subsysName)
		return nil, err
	}
	ctrlrID := autoCtrlrIDAllocation
	if in.NvmeController.Spec.NvmeControllerId != nil {
		ctrlrID = int(*in.NvmeController.Spec.NvmeControllerId)
	}
	// construct command with parameters
	params := models.MrvlNvmSubsysCreateCtrlrParams{
		Subnqn:       subsys.Spec.Nqn,
		PcieDomainID: int(in.GetNvmeController().GetSpec().GetPcieId().GetPortId().GetValue()),
		PfID:         int(in.GetNvmeController().GetSpec().GetPcieId().GetPhysicalFunction().GetValue()),
		VfID:         int(in.GetNvmeController().GetSpec().GetPcieId().GetVirtualFunction().GetValue()),
		CtrlrID:      ctrlrID,
		MaxNsq:       int(in.GetNvmeController().GetSpec().GetMaxNsq()),
		MaxNcq:       int(in.GetNvmeController().GetSpec().GetMaxNcq()),
		Mqes:         int(in.GetNvmeController().GetSpec().GetSqes()),
	}
	var result models.MrvlNvmSubsysCreateCtrlrResult
	err = s.rpc.Call(ctx, "mrvl_nvm_subsys_update_ctrlr", &params, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		msg := fmt.Sprintf("Could not update CTRL: %s", in.NvmeController.Name)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	response := utils.ProtoClone(in.NvmeController)
	response.Spec.NvmeControllerId = proto.Int32(int32(result.CtrlrID))
	response.Status = &pb.NvmeControllerStatus{Active: true}
	err = s.store.Set(in.NvmeController.Name, response)
	if err != nil {
		return nil, err
	}
	return response, nil
}

// ListNvmeControllers lists Nvme controllers
func (s *Server) ListNvmeControllers(ctx context.Context, in *pb.ListNvmeControllersRequest) (*pb.ListNvmeControllersResponse, error) {
	// check required fields
	if err := fieldbehavior.ValidateRequiredFields(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	size, offset, perr := utils.ExtractPagination(in.PageSize, in.PageToken, s.Pagination)
	if perr != nil {
		return nil, perr
	}
	subsys := new(pb.NvmeSubsystem)
	found, err := s.store.Get(in.Parent, subsys)
	if err != nil {
		return nil, err
	}
	if !found {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Parent)
		return nil, err
	}
	params := models.MrvlNvmSubsysGetCtrlrListParams{
		Subnqn: subsys.Spec.Nqn,
	}
	var result models.MrvlNvmSubsysGetCtrlrListResult
	err = s.rpc.Call(ctx, "mrvl_nvm_subsys_get_ctrlr_list", &params, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		msg := fmt.Sprintf("Could not list CTRLs: %v", in.Parent)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	token, hasMoreElements := "", false
	log.Printf("Limiting result len(%d) to [%d:%d]", len(result.CtrlrIDList), offset, size)
	result.CtrlrIDList, hasMoreElements = utils.LimitPagination(result.CtrlrIDList, offset, size)
	if hasMoreElements {
		token = uuid.New().String()
		s.Pagination[token] = offset + size
	}
	Blobarray := make([]*pb.NvmeController, len(result.CtrlrIDList))
	for i := range result.CtrlrIDList {
		r := &result.CtrlrIDList[i]
		Blobarray[i] = &pb.NvmeController{Spec: &pb.NvmeControllerSpec{NvmeControllerId: proto.Int32(int32(r.CtrlrID))}}
	}
	sortNvmeControllers(Blobarray)
	return &pb.ListNvmeControllersResponse{NvmeControllers: Blobarray}, nil
}

// GetNvmeController gets an Nvme controller
func (s *Server) GetNvmeController(ctx context.Context, in *pb.GetNvmeControllerRequest) (*pb.NvmeController, error) {
	// check input correctness
	if err := s.validateGetNvmeControllerRequest(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	controller := new(pb.NvmeController)
	found, err := s.store.Get(in.Name, controller)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("error finding controller %s", in.Name)
	}
	subsysName := utils.ResourceIDToSubsystemName(
		utils.GetSubsystemIDFromNvmeName(in.Name),
	)
	// fetch object from the database
	subsys := new(pb.NvmeSubsystem)
	found, err = s.store.Get(subsysName, subsys)
	if err != nil {
		return nil, err
	}
	if !found {
		err := status.Errorf(codes.NotFound, "unable to find key %s", subsysName)
		return nil, err
	}

	params := models.MrvlNvmGetCtrlrInfoParams{
		Subnqn:  subsys.Spec.Nqn,
		CtrlrID: int(*controller.Spec.NvmeControllerId),
	}
	var result models.MrvlNvmGetCtrlrInfoResult
	err = s.rpc.Call(ctx, "mrvl_nvm_ctrlr_get_info", &params, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		msg := fmt.Sprintf("Could not get CTRL: %s", in.Name)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}

	return &pb.NvmeController{Name: in.Name, Spec: &pb.NvmeControllerSpec{NvmeControllerId: controller.Spec.NvmeControllerId}, Status: &pb.NvmeControllerStatus{Active: true}}, nil
}

// StatsNvmeController gets an Nvme controller stats
func (s *Server) StatsNvmeController(ctx context.Context, in *pb.StatsNvmeControllerRequest) (*pb.StatsNvmeControllerResponse, error) {
	// check input correctness
	if err := s.validateStatsNvmeControllerRequest(in); err != nil {
		return nil, err
	}
	// fetch object from the database
	controller := new(pb.NvmeController)
	found, err := s.store.Get(in.Name, controller)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("error finding controller %s", in.Name)
	}
	subsysName := utils.ResourceIDToSubsystemName(
		utils.GetSubsystemIDFromNvmeName(in.Name),
	)
	// fetch object from the database
	subsys := new(pb.NvmeSubsystem)
	found, err = s.store.Get(subsysName, subsys)
	if err != nil {
		return nil, err
	}
	if !found {
		err := status.Errorf(codes.NotFound, "unable to find key %s", subsysName)
		return nil, err
	}

	params := models.MrvlNvmGetCtrlrStatsParams{
		Subnqn:  subsys.Spec.Nqn,
		CtrlrID: int(*controller.Spec.NvmeControllerId),
	}
	var result models.MrvlNvmGetCtrlrStatsResult
	err = s.rpc.Call(ctx, "mrvl_nvm_get_ctrlr_stats", &params, &result)
	if err != nil {
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		msg := fmt.Sprintf("Could not stats CTRL: %s", in.Name)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	return &pb.StatsNvmeControllerResponse{Stats: &pb.VolumeStats{
		ReadBytesCount:    int32(result.NumReadBytes),
		ReadOpsCount:      int32(result.NumReadCmds),
		WriteBytesCount:   int32(result.NumWriteBytes),
		WriteOpsCount:     int32(result.NumWriteCmds),
		ReadLatencyTicks:  int32(result.TotalReadLatencyInUs),
		WriteLatencyTicks: int32(result.TotalWriteLatencyInUs),
	}}, nil
}
