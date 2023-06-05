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
	"sort"
	"strconv"

	"github.com/opiproject/gospdk/spdk"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-marvell-bridge/pkg/models"
	"github.com/opiproject/opi-spdk-bridge/pkg/server"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Server contains frontend related OPI services
type Server struct {
	pb.UnimplementedFrontendNvmeServiceServer
	Subsystems  map[string]*pb.NvmeSubsystem
	Controllers map[string]*pb.NvmeController
	Namespaces  map[string]*pb.NvmeNamespace
	Pagination  map[string]int
	rpc         spdk.JSONRPC
}

// NewServer creates initialized instance of Nvme server
func NewServer(jsonRPC spdk.JSONRPC) *Server {
	return &Server{
		Subsystems:  make(map[string]*pb.NvmeSubsystem),
		Controllers: make(map[string]*pb.NvmeController),
		Namespaces:  make(map[string]*pb.NvmeNamespace),
		Pagination:  make(map[string]int),
		rpc:         jsonRPC,
	}
}

func sortNvmeSubsystems(subsystems []*pb.NvmeSubsystem) {
	sort.Slice(subsystems, func(i int, j int) bool {
		return subsystems[i].Spec.Nqn < subsystems[j].Spec.Nqn
	})
}

func sortNvmeControllers(controllers []*pb.NvmeController) {
	sort.Slice(controllers, func(i int, j int) bool {
		return controllers[i].Spec.NvmeControllerId < controllers[j].Spec.NvmeControllerId
	})
}

func sortNvmeNamespaces(namespaces []*pb.NvmeNamespace) {
	sort.Slice(namespaces, func(i int, j int) bool {
		return namespaces[i].Spec.HostNsid < namespaces[j].Spec.HostNsid
	})
}

// CreateNvmeSubsystem creates an Nvme Subsystem
func (s *Server) CreateNvmeSubsystem(_ context.Context, in *pb.CreateNvmeSubsystemRequest) (*pb.NvmeSubsystem, error) {
	log.Printf("CreateNvmeSubsystem: Received from client: %v", in)
	// see https://google.aip.dev/133#user-specified-ids
	resourceID := uuid.New().String()
	if in.NvmeSubsystemId != "" {
		log.Printf("client provided the ID of a resource %v, ignoring the name field %v", in.NvmeSubsystemId, in.NvmeSubsystem.Name)
		resourceID = in.NvmeSubsystemId
	}
	in.NvmeSubsystem.Name = resourceID
	// idempotent API when called with same key, should return same object
	subsys, ok := s.Subsystems[in.NvmeSubsystem.Name]
	if ok {
		log.Printf("Already existing NvmeSubsystem with id %v", in.NvmeSubsystem.Name)
		return subsys, nil
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
	err := s.rpc.Call("mrvl_nvm_create_subsystem", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		msg := fmt.Sprintf("Could not create NQN: %s", in.NvmeSubsystem.Spec.Nqn)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	var ver spdk.GetVersionResult
	err = s.rpc.Call("spdk_get_version", nil, &ver)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", ver)
	response := server.ProtoClone(in.NvmeSubsystem)
	response.Status = &pb.NvmeSubsystemStatus{FirmwareRevision: ver.Version}
	s.Subsystems[in.NvmeSubsystem.Name] = response
	return response, nil
}

// DeleteNvmeSubsystem deletes an Nvme Subsystem
func (s *Server) DeleteNvmeSubsystem(_ context.Context, in *pb.DeleteNvmeSubsystemRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteNvmeSubsystem: Received from client: %v", in)
	subsys, ok := s.Subsystems[in.Name]
	if !ok {
		if in.AllowMissing {
			return &emptypb.Empty{}, nil
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}
	params := models.MrvlNvmDeleteSubsystemParams{
		Subnqn: subsys.Spec.Nqn,
	}
	var result models.MrvlNvmDeleteSubsystemResult
	err := s.rpc.Call("mrvl_nvm_delete_subsystem", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		msg := fmt.Sprintf("Could not delete NQN: %s", subsys.Spec.Nqn)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	delete(s.Subsystems, subsys.Name)
	return &emptypb.Empty{}, nil
}

// UpdateNvmeSubsystem updates an Nvme Subsystem
func (s *Server) UpdateNvmeSubsystem(_ context.Context, in *pb.UpdateNvmeSubsystemRequest) (*pb.NvmeSubsystem, error) {
	log.Printf("UpdateNvmeSubsystem: Received from client: %v", in)
	return nil, status.Errorf(codes.Unimplemented, "UpdateNvmeSubsystem method is not implemented")
}

// ListNvmeSubsystems lists Nvme Subsystems
func (s *Server) ListNvmeSubsystems(_ context.Context, in *pb.ListNvmeSubsystemsRequest) (*pb.ListNvmeSubsystemsResponse, error) {
	log.Printf("ListNvmeSubsystems: Received from client: %v", in)
	size, offset, perr := server.ExtractPagination(in.PageSize, in.PageToken, s.Pagination)
	if perr != nil {
		log.Printf("error: %v", perr)
		return nil, perr
	}
	var result models.MrvlNvmGetSubsysListResult
	err := s.rpc.Call("mrvl_nvm_get_subsys_list", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		msg := "Could not list subsystems"
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	token, hasMoreElements := "", false
	log.Printf("Limiting result len(%d) to [%d:%d]", len(result.SubsysList), offset, size)
	result.SubsysList, hasMoreElements = server.LimitPagination(result.SubsysList, offset, size)
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
func (s *Server) GetNvmeSubsystem(_ context.Context, in *pb.GetNvmeSubsystemRequest) (*pb.NvmeSubsystem, error) {
	log.Printf("GetNvmeSubsystem: Received from client: %v", in)
	subsys, ok := s.Subsystems[in.Name]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}
	// TODO: replace with MRVL code : mrvl_nvm_subsys_get_info ?
	var result models.MrvlNvmGetSubsysListResult
	err := s.rpc.Call("mrvl_nvm_get_subsys_list", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		msg := fmt.Sprintf("Could not list NQN: %s", subsys.Spec.Nqn)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	for i := range result.SubsysList {
		r := &result.SubsysList[i]
		if r.Subnqn == subsys.Spec.Nqn {
			return &pb.NvmeSubsystem{Spec: &pb.NvmeSubsystemSpec{Nqn: r.Subnqn}, Status: &pb.NvmeSubsystemStatus{FirmwareRevision: "TBD"}}, nil
		}
	}
	msg := fmt.Sprintf("Could not find NQN: %s", subsys.Spec.Nqn)
	log.Print(msg)
	return nil, status.Errorf(codes.InvalidArgument, msg)
}

// NvmeSubsystemStats gets Nvme Subsystem stats
func (s *Server) NvmeSubsystemStats(_ context.Context, in *pb.NvmeSubsystemStatsRequest) (*pb.NvmeSubsystemStatsResponse, error) {
	log.Printf("NvmeSubsystemStats: Received from client: %v", in)
	subsys, ok := s.Subsystems[in.SubsystemId.Value]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.SubsystemId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}
	params := models.MrvlNvmGetSubsysInfoParams{
		Subnqn: subsys.Spec.Nqn,
	}
	var result models.MrvlNvmGetSubsysInfoResult
	err := s.rpc.Call("mrvl_nvm_subsys_get_info", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		msg := fmt.Sprintf("Could not stats NQN: %s", subsys.Spec.Nqn)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	return &pb.NvmeSubsystemStatsResponse{Stats: &pb.VolumeStats{ReadOpsCount: -1, WriteOpsCount: -1}}, nil
}

// CreateNvmeController creates an Nvme controller
func (s *Server) CreateNvmeController(_ context.Context, in *pb.CreateNvmeControllerRequest) (*pb.NvmeController, error) {
	log.Printf("CreateNvmeController: Received from client: %v", in)
	// check input parameters validity
	if in.NvmeController.Spec == nil || in.NvmeController.Spec.SubsystemId == nil || in.NvmeController.Spec.SubsystemId.Value == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid input subsystem parameters")
	}
	// see https://google.aip.dev/133#user-specified-ids
	resourceID := uuid.New().String()
	if in.NvmeControllerId != "" {
		log.Printf("client provided the ID of a resource %v, ignoring the name field %v", in.NvmeControllerId, in.NvmeController.Name)
		resourceID = in.NvmeControllerId
	}
	in.NvmeController.Name = resourceID
	// idempotent API when called with same key, should return same object
	controller, ok := s.Controllers[in.NvmeController.Name]
	if ok {
		log.Printf("Already existing NvmeController with id %v", in.NvmeController.Name)
		return controller, nil
	}
	// not found, so create a new one
	subsys, ok := s.Subsystems[in.NvmeController.Spec.SubsystemId.Value]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.NvmeController.Spec.SubsystemId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}

	params := models.MrvlNvmSubsysCreateCtrlrParams{
		Subnqn:       subsys.Spec.Nqn,
		PcieDomainID: int(in.NvmeController.Spec.PcieId.PortId),
		PfID:         int(in.NvmeController.Spec.PcieId.PhysicalFunction),
		VfID:         int(in.NvmeController.Spec.PcieId.VirtualFunction),
		CtrlrID:      int(in.NvmeController.Spec.NvmeControllerId),
		MaxNsq:       int(in.NvmeController.Spec.MaxNsq),
		MaxNcq:       int(in.NvmeController.Spec.MaxNcq),
		Mqes:         int(in.NvmeController.Spec.Sqes),
	}
	var result models.MrvlNvmSubsysCreateCtrlrResult
	err := s.rpc.Call("mrvl_nvm_subsys_create_ctrlr", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		msg := fmt.Sprintf("Could not create CTRL: %s", in.NvmeController.Name)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	s.Controllers[in.NvmeController.Name] = in.NvmeController
	s.Controllers[in.NvmeController.Name].Spec.NvmeControllerId = int32(result.CtrlrID)
	s.Controllers[in.NvmeController.Name].Status = &pb.NvmeControllerStatus{Active: true}
	response := server.ProtoClone(in.NvmeController)
	return response, nil
}

// DeleteNvmeController deletes an Nvme controller
func (s *Server) DeleteNvmeController(_ context.Context, in *pb.DeleteNvmeControllerRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteNvmeController: Received from client: %v", in)
	controller, ok := s.Controllers[in.Name]
	if !ok {
		if in.AllowMissing {
			return &emptypb.Empty{}, nil
		}
		return nil, fmt.Errorf("error finding controller %s", in.Name)
	}
	subsys, ok := s.Subsystems[controller.Spec.SubsystemId.Value]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", controller.Spec.SubsystemId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}

	params := models.MrvlNvmSubsysRemoveCtrlrParams{
		Subnqn:  subsys.Spec.Nqn,
		CtrlrID: int(controller.Spec.NvmeControllerId),
		Force:   1,
	}
	var result models.MrvlNvmSubsysRemoveCtrlrResult
	err := s.rpc.Call("mrvl_nvm_subsys_remove_ctrlr", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		msg := fmt.Sprintf("Could not delete CTRL: %s", controller.Name)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	delete(s.Controllers, controller.Name)
	return &emptypb.Empty{}, nil
}

// UpdateNvmeController updates an Nvme controller
func (s *Server) UpdateNvmeController(_ context.Context, in *pb.UpdateNvmeControllerRequest) (*pb.NvmeController, error) {
	log.Printf("UpdateNvmeController: Received from client: %v", in)
	subsys, ok := s.Subsystems[in.NvmeController.Spec.SubsystemId.Value]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.NvmeController.Spec.SubsystemId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}
	params := models.MrvlNvmSubsysCreateCtrlrParams{
		Subnqn:       subsys.Spec.Nqn,
		PcieDomainID: int(in.NvmeController.Spec.PcieId.PortId),
		PfID:         int(in.NvmeController.Spec.PcieId.PhysicalFunction),
		VfID:         int(in.NvmeController.Spec.PcieId.VirtualFunction),
		CtrlrID:      int(in.NvmeController.Spec.NvmeControllerId),
		MaxNsq:       int(in.NvmeController.Spec.MaxNsq),
		MaxNcq:       int(in.NvmeController.Spec.MaxNcq),
		Mqes:         int(in.NvmeController.Spec.Sqes),
	}
	var result models.MrvlNvmSubsysCreateCtrlrResult
	err := s.rpc.Call("mrvl_nvm_subsys_update_ctrlr", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		msg := fmt.Sprintf("Could not update CTRL: %s", in.NvmeController.Name)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	s.Controllers[in.NvmeController.Name] = in.NvmeController
	s.Controllers[in.NvmeController.Name].Spec.NvmeControllerId = int32(result.CtrlrID)
	s.Controllers[in.NvmeController.Name].Status = &pb.NvmeControllerStatus{Active: true}
	response := server.ProtoClone(in.NvmeController)
	return response, nil
}

// ListNvmeControllers lists Nvme controllers
func (s *Server) ListNvmeControllers(_ context.Context, in *pb.ListNvmeControllersRequest) (*pb.ListNvmeControllersResponse, error) {
	log.Printf("ListNvmeControllers: Received from client: %v", in)
	size, offset, perr := server.ExtractPagination(in.PageSize, in.PageToken, s.Pagination)
	if perr != nil {
		log.Printf("error: %v", perr)
		return nil, perr
	}
	subsys, ok := s.Subsystems[in.Parent]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Parent)
		log.Printf("error: %v", err)
		return nil, err
	}
	params := models.MrvlNvmSubsysGetCtrlrListParams{
		Subnqn: subsys.Spec.Nqn,
	}
	var result models.MrvlNvmSubsysGetCtrlrListResult
	err := s.rpc.Call("mrvl_nvm_subsys_get_ctrlr_list", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		msg := fmt.Sprintf("Could not list CTRLs: %v", in.Parent)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	token, hasMoreElements := "", false
	log.Printf("Limiting result len(%d) to [%d:%d]", len(result.CtrlrIDList), offset, size)
	result.CtrlrIDList, hasMoreElements = server.LimitPagination(result.CtrlrIDList, offset, size)
	if hasMoreElements {
		token = uuid.New().String()
		s.Pagination[token] = offset + size
	}
	Blobarray := make([]*pb.NvmeController, len(result.CtrlrIDList))
	for i := range result.CtrlrIDList {
		r := &result.CtrlrIDList[i]
		Blobarray[i] = &pb.NvmeController{Spec: &pb.NvmeControllerSpec{NvmeControllerId: int32(r.CtrlrID)}}
	}
	sortNvmeControllers(Blobarray)
	return &pb.ListNvmeControllersResponse{NvmeControllers: Blobarray}, nil
}

// GetNvmeController gets an Nvme controller
func (s *Server) GetNvmeController(_ context.Context, in *pb.GetNvmeControllerRequest) (*pb.NvmeController, error) {
	log.Printf("GetNvmeController: Received from client: %v", in)
	controller, ok := s.Controllers[in.Name]
	if !ok {
		return nil, fmt.Errorf("error finding controller %s", in.Name)
	}
	subsys, ok := s.Subsystems[controller.Spec.SubsystemId.Value]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", controller.Spec.SubsystemId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}

	params := models.MrvlNvmGetCtrlrInfoParams{
		Subnqn:  subsys.Spec.Nqn,
		CtrlrID: int(controller.Spec.NvmeControllerId),
	}
	var result models.MrvlNvmGetCtrlrInfoResult
	err := s.rpc.Call("mrvl_nvm_ctrlr_get_info", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		msg := fmt.Sprintf("Could not get CTRL: %s", in.Name)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}

	return &pb.NvmeController{Name: in.Name, Spec: &pb.NvmeControllerSpec{NvmeControllerId: controller.Spec.NvmeControllerId}, Status: &pb.NvmeControllerStatus{Active: true}}, nil
}

// NvmeControllerStats gets an Nvme controller stats
func (s *Server) NvmeControllerStats(_ context.Context, in *pb.NvmeControllerStatsRequest) (*pb.NvmeControllerStatsResponse, error) {
	log.Printf("NvmeControllerStats: Received from client: %v", in)
	controller, ok := s.Controllers[in.Id.Value]
	if !ok {
		return nil, fmt.Errorf("error finding controller %s", in.Id.Value)
	}
	subsys, ok := s.Subsystems[controller.Spec.SubsystemId.Value]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", controller.Spec.SubsystemId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}

	params := models.MrvlNvmGetCtrlrStatsParams{
		Subnqn:  subsys.Spec.Nqn,
		CtrlrID: int(controller.Spec.NvmeControllerId),
	}
	var result models.MrvlNvmGetCtrlrStatsResult
	err := s.rpc.Call("mrvl_nvm_get_ctrlr_stats", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		msg := fmt.Sprintf("Could not stats CTRL: %s", in.Id.Value)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	return &pb.NvmeControllerStatsResponse{Stats: &pb.VolumeStats{
		ReadBytesCount:    int32(result.NumReadBytes),
		ReadOpsCount:      int32(result.NumReadCmds),
		WriteBytesCount:   int32(result.NumWriteBytes),
		WriteOpsCount:     int32(result.NumWriteCmds),
		ReadLatencyTicks:  int32(result.TotalReadLatencyInUs),
		WriteLatencyTicks: int32(result.TotalWriteLatencyInUs),
	}}, nil
}

// CreateNvmeNamespace creates an Nvme namespace
func (s *Server) CreateNvmeNamespace(_ context.Context, in *pb.CreateNvmeNamespaceRequest) (*pb.NvmeNamespace, error) {
	log.Printf("CreateNvmeNamespace: Received from client: %v", in)
	// check input parameters validity
	if in.NvmeNamespace.Spec == nil || in.NvmeNamespace.Spec.SubsystemId == nil || in.NvmeNamespace.Spec.SubsystemId.Value == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid input subsystem parameters")
	}
	// see https://google.aip.dev/133#user-specified-ids
	resourceID := uuid.New().String()
	if in.NvmeNamespaceId != "" {
		log.Printf("client provided the ID of a resource %v, ignoring the name field %v", in.NvmeNamespaceId, in.NvmeNamespace.Name)
		resourceID = in.NvmeNamespaceId
	}
	in.NvmeNamespace.Name = resourceID
	// idempotent API when called with same key, should return same object
	namespace, ok := s.Namespaces[in.NvmeNamespace.Name]
	if ok {
		log.Printf("Already existing NvmeNamespace with id %v", in.NvmeNamespace.Name)
		return namespace, nil
	}
	// not found, so create a new one
	subsys, ok := s.Subsystems[in.NvmeNamespace.Spec.SubsystemId.Value]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.NvmeNamespace.Spec.SubsystemId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}
	// TODO: do lookup through VolumeId key instead of using it's value
	params := models.MrvlNvmSubsysAllocNsParams{
		Subnqn:      subsys.Spec.Nqn,
		Nguid:       in.NvmeNamespace.Spec.Nguid,
		Eui64:       strconv.FormatInt(in.NvmeNamespace.Spec.Eui64, 10),
		UUID:        in.NvmeNamespace.Spec.Uuid.Value,
		ShareEnable: 1,
		Bdev:        in.NvmeNamespace.Spec.VolumeId.Value,
	}
	var result models.MrvlNvmSubsysAllocNsResult
	err := s.rpc.Call("mrvl_nvm_subsys_alloc_ns", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		msg := fmt.Sprintf("Could not create NS: %s", in.NvmeNamespace.Name)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}

	s.Namespaces[in.NvmeNamespace.Name] = in.NvmeNamespace

	// Now, attach this new NS to ALL controllers
	for _, c := range s.Controllers {
		if c.Spec.SubsystemId.Value != in.NvmeNamespace.Spec.SubsystemId.Value {
			continue
		}
		params := models.MrvlNvmCtrlrAttachNsParams{
			Subnqn:       subsys.Spec.Nqn,
			CtrlrID:      int(c.Spec.NvmeControllerId),
			NsInstanceID: int(in.NvmeNamespace.Spec.HostNsid),
		}
		var result models.MrvlNvmCtrlrAttachNsResult
		err := s.rpc.Call("mrvl_nvm_ctrlr_attach_ns", &params, &result)
		if err != nil {
			log.Printf("error: %v", err)
			return nil, err
		}
		if result.Status != 0 {
			msg := fmt.Sprintf("Could not attach NS: %s", in.NvmeNamespace.Name)
			log.Print(msg)
			return nil, status.Errorf(codes.InvalidArgument, msg)
		}
	}
	response := server.ProtoClone(in.NvmeNamespace)
	response.Status = &pb.NvmeNamespaceStatus{PciState: 2, PciOperState: 1}
	return response, nil
}

// DeleteNvmeNamespace deletes an Nvme namespace
func (s *Server) DeleteNvmeNamespace(_ context.Context, in *pb.DeleteNvmeNamespaceRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteNvmeNamespace: Received from client: %v", in)
	namespace, ok := s.Namespaces[in.Name]
	if !ok {
		if in.AllowMissing {
			return &emptypb.Empty{}, nil
		}
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}
	subsys, ok := s.Subsystems[namespace.Spec.SubsystemId.Value]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find subsystem %s", namespace.Spec.SubsystemId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}
	// First, detach this NS from ALL controllers
	for _, c := range s.Controllers {
		if c.Spec.SubsystemId.Value != namespace.Spec.SubsystemId.Value {
			continue
		}
		params := models.MrvlNvmCtrlrDetachNsParams{
			Subnqn:       subsys.Spec.Nqn,
			CtrlrID:      int(c.Spec.NvmeControllerId),
			NsInstanceID: int(namespace.Spec.HostNsid),
		}
		var result models.MrvlNvmCtrlrDetachNsResult
		err := s.rpc.Call("mrvl_nvm_ctrlr_detach_ns", &params, &result)
		if err != nil {
			log.Printf("error: %v", err)
			return nil, err
		}
		log.Printf("Received from SPDK: %v", result)
		if result.Status != 0 {
			msg := fmt.Sprintf("Could not detach NS: %s", in.Name)
			log.Print(msg)
			return nil, status.Errorf(codes.InvalidArgument, msg)
		}
	}
	params := models.MrvlNvmSubsysUnallocNsParams{
		Subnqn:       subsys.Spec.Nqn,
		NsInstanceID: int(namespace.Spec.HostNsid),
	}
	var result models.MrvlNvmSubsysUnallocNsResult
	err := s.rpc.Call("mrvl_nvm_subsys_unalloc_ns", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		msg := fmt.Sprintf("Could not delete NS: %s", in.Name)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	delete(s.Namespaces, namespace.Name)
	return &emptypb.Empty{}, nil
}

// UpdateNvmeNamespace updates an Nvme namespace
func (s *Server) UpdateNvmeNamespace(_ context.Context, in *pb.UpdateNvmeNamespaceRequest) (*pb.NvmeNamespace, error) {
	log.Printf("UpdateNvmeNamespace: Received from client: %v", in)
	return nil, status.Errorf(codes.Unimplemented, "UpdateNvmeNamespace method is not implemented")
}

// ListNvmeNamespaces lists Nvme namespaces
func (s *Server) ListNvmeNamespaces(_ context.Context, in *pb.ListNvmeNamespacesRequest) (*pb.ListNvmeNamespacesResponse, error) {
	log.Printf("ListNvmeNamespaces: Received from client: %v", in)
	size, offset, perr := server.ExtractPagination(in.PageSize, in.PageToken, s.Pagination)
	if perr != nil {
		log.Printf("error: %v", perr)
		return nil, perr
	}
	subsys, ok := s.Subsystems[in.Parent]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Parent)
		log.Printf("error: %v", err)
		return nil, err
	}
	params := models.MrvlNvmSubsysGetNsListParams{
		Subnqn: subsys.Spec.Nqn,
	}
	var result models.MrvlNvmSubsysGetNsListResult
	err := s.rpc.Call("mrvl_nvm_subsys_get_ns_list", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		msg := fmt.Sprintf("Could not list NS: %s", in.Parent)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	token, hasMoreElements := "", false
	log.Printf("Limiting result len(%d) to [%d:%d]", len(result.NsList), offset, size)
	result.NsList, hasMoreElements = server.LimitPagination(result.NsList, offset, size)
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
func (s *Server) GetNvmeNamespace(_ context.Context, in *pb.GetNvmeNamespaceRequest) (*pb.NvmeNamespace, error) {
	log.Printf("GetNvmeNamespace: Received from client: %v", in)
	namespace, ok := s.Namespaces[in.Name]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("error: %v", namespace)
	subsys, ok := s.Subsystems[namespace.Spec.SubsystemId.Value]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", namespace.Spec.SubsystemId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}

	params := models.MrvlNvmGetNsInfoParams{
		SubNqn:       subsys.Spec.Nqn,
		NsInstanceID: int(namespace.Spec.HostNsid),
	}
	var result models.MrvlNvmGetNsInfoResult
	err := s.rpc.Call("mrvl_nvm_ns_get_info", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		msg := fmt.Sprintf("Could not get NS: %s", in.Name)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	return &pb.NvmeNamespace{Name: in.Name, Spec: &pb.NvmeNamespaceSpec{Nguid: result.Nguid}, Status: &pb.NvmeNamespaceStatus{PciState: 2, PciOperState: 1}}, nil
}

// NvmeNamespaceStats gets an Nvme namespace stats
func (s *Server) NvmeNamespaceStats(_ context.Context, in *pb.NvmeNamespaceStatsRequest) (*pb.NvmeNamespaceStatsResponse, error) {
	log.Printf("NvmeNamespaceStats: Received from client: %v", in)
	namespace, ok := s.Namespaces[in.NamespaceId.Value]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.NamespaceId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}
	subsys, ok := s.Subsystems[namespace.Spec.SubsystemId.Value]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", namespace.Spec.SubsystemId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}

	params := models.MrvlNvmGetNsStatsParams{
		SubNqn:       subsys.Spec.Nqn,
		NsInstanceID: int(namespace.Spec.HostNsid),
	}
	var result models.MrvlNvmGetNsStatsResult
	err := s.rpc.Call("mrvl_nvm_get_ns_stats", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		msg := fmt.Sprintf("Could not stats NS: %s", in.NamespaceId.Value)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	return &pb.NvmeNamespaceStatsResponse{Stats: &pb.VolumeStats{
		ReadBytesCount:    int32(result.NumReadBytes),
		ReadOpsCount:      int32(result.NumReadCmds),
		WriteBytesCount:   int32(result.NumWriteBytes),
		WriteOpsCount:     int32(result.NumWriteCmds),
		ReadLatencyTicks:  int32(result.TotalReadLatencyInUs),
		WriteLatencyTicks: int32(result.TotalWriteLatencyInUs),
	}}, nil
}
