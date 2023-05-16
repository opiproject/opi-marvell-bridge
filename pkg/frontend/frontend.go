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
	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/opiproject/opi-marvell-bridge/pkg/models"
	"github.com/opiproject/opi-spdk-bridge/pkg/server"

	"github.com/google/uuid"
	"github.com/ulule/deepcopier"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Server contains frontend related OPI services
type Server struct {
	pb.UnimplementedFrontendNvmeServiceServer
	Subsystems  map[string]*pb.NVMeSubsystem
	Controllers map[string]*pb.NVMeController
	Namespaces  map[string]*pb.NVMeNamespace
	Pagination  map[string]int
	rpc         spdk.JSONRPC
}

// NewServer creates initialized instance of NVMe server
func NewServer(jsonRPC spdk.JSONRPC) *Server {
	return &Server{
		Subsystems:  make(map[string]*pb.NVMeSubsystem),
		Controllers: make(map[string]*pb.NVMeController),
		Namespaces:  make(map[string]*pb.NVMeNamespace),
		Pagination:  make(map[string]int),
		rpc:         jsonRPC,
	}
}

func sortNVMeSubsystems(subsystems []*pb.NVMeSubsystem) {
	sort.Slice(subsystems, func(i int, j int) bool {
		return subsystems[i].Spec.Nqn < subsystems[j].Spec.Nqn
	})
}

func sortNVMeControllers(controllers []*pb.NVMeController) {
	sort.Slice(controllers, func(i int, j int) bool {
		return controllers[i].Spec.NvmeControllerId < controllers[j].Spec.NvmeControllerId
	})
}

func sortNVMeNamespaces(namespaces []*pb.NVMeNamespace) {
	sort.Slice(namespaces, func(i int, j int) bool {
		return namespaces[i].Spec.HostNsid < namespaces[j].Spec.HostNsid
	})
}

// CreateNVMeSubsystem creates an NVMe Subsystem
func (s *Server) CreateNVMeSubsystem(_ context.Context, in *pb.CreateNVMeSubsystemRequest) (*pb.NVMeSubsystem, error) {
	log.Printf("CreateNVMeSubsystem: Received from client: %v", in)
	// see https://google.aip.dev/133#user-specified-ids
	if in.NvMeSubsystemId != "" {
		log.Printf("client provided the ID of a resource %v, ignoring the name field %v", in.NvMeSubsystemId, in.NvMeSubsystem.Spec.Id.Value)
	}
	// idempotent API when called with same key, should return same object
	subsys, ok := s.Subsystems[in.NvMeSubsystem.Spec.Id.Value]
	if ok {
		log.Printf("Already existing NVMeSubsystem with id %v", in.NvMeSubsystem.Spec.Id.Value)
		return subsys, nil
	}
	// not found, so create a new one

	// TODO: fix const values below
	params := models.MrvlNvmCreateSubsystemParams{
		Subnqn:        in.NvMeSubsystem.Spec.Nqn,
		Mn:            in.NvMeSubsystem.Spec.ModelNumber,
		Sn:            in.NvMeSubsystem.Spec.SerialNumber,
		MaxNamespaces: int(in.NvMeSubsystem.Spec.MaxNamespaces),
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
		msg := fmt.Sprintf("Could not create NQN: %s", in.NvMeSubsystem.Spec.Nqn)
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
	response := &pb.NVMeSubsystem{}
	err = deepcopier.Copy(in.NvMeSubsystem).To(response)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	response.Status = &pb.NVMeSubsystemStatus{FirmwareRevision: ver.Version}
	s.Subsystems[in.NvMeSubsystem.Spec.Id.Value] = response
	return response, nil
}

// DeleteNVMeSubsystem deletes an NVMe Subsystem
func (s *Server) DeleteNVMeSubsystem(_ context.Context, in *pb.DeleteNVMeSubsystemRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteNVMeSubsystem: Received from client: %v", in)
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
	delete(s.Subsystems, subsys.Spec.Id.Value)
	return &emptypb.Empty{}, nil
}

// UpdateNVMeSubsystem updates an NVMe Subsystem
func (s *Server) UpdateNVMeSubsystem(_ context.Context, in *pb.UpdateNVMeSubsystemRequest) (*pb.NVMeSubsystem, error) {
	log.Printf("UpdateNVMeSubsystem: Received from client: %v", in)
	return nil, status.Errorf(codes.Unimplemented, "UpdateNVMeSubsystem method is not implemented")
}

// ListNVMeSubsystems lists NVMe Subsystems
func (s *Server) ListNVMeSubsystems(_ context.Context, in *pb.ListNVMeSubsystemsRequest) (*pb.ListNVMeSubsystemsResponse, error) {
	log.Printf("ListNVMeSubsystems: Received from client: %v", in)
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
	Blobarray := make([]*pb.NVMeSubsystem, len(result.SubsysList))
	for i := range result.SubsysList {
		r := &result.SubsysList[i]
		Blobarray[i] = &pb.NVMeSubsystem{Spec: &pb.NVMeSubsystemSpec{Nqn: r.Subnqn}}
	}
	sortNVMeSubsystems(Blobarray)
	return &pb.ListNVMeSubsystemsResponse{NvMeSubsystems: Blobarray}, nil
}

// GetNVMeSubsystem gets NVMe Subsystems
func (s *Server) GetNVMeSubsystem(_ context.Context, in *pb.GetNVMeSubsystemRequest) (*pb.NVMeSubsystem, error) {
	log.Printf("GetNVMeSubsystem: Received from client: %v", in)
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
			return &pb.NVMeSubsystem{Spec: &pb.NVMeSubsystemSpec{Nqn: r.Subnqn}, Status: &pb.NVMeSubsystemStatus{FirmwareRevision: "TBD"}}, nil
		}
	}
	msg := fmt.Sprintf("Could not find NQN: %s", subsys.Spec.Nqn)
	log.Print(msg)
	return nil, status.Errorf(codes.InvalidArgument, msg)
}

// NVMeSubsystemStats gets NVMe Subsystem stats
func (s *Server) NVMeSubsystemStats(_ context.Context, in *pb.NVMeSubsystemStatsRequest) (*pb.NVMeSubsystemStatsResponse, error) {
	log.Printf("NVMeSubsystemStats: Received from client: %v", in)
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
	return &pb.NVMeSubsystemStatsResponse{Stats: &pb.VolumeStats{ReadOpsCount: -1, WriteOpsCount: -1}}, nil
}

// CreateNVMeController creates an NVMe controller
func (s *Server) CreateNVMeController(_ context.Context, in *pb.CreateNVMeControllerRequest) (*pb.NVMeController, error) {
	log.Printf("CreateNVMeController: Received from client: %v", in)
	// see https://google.aip.dev/133#user-specified-ids
	if in.NvMeControllerId != "" {
		log.Printf("client provided the ID of a resource %v, ignoring the name field %v", in.NvMeControllerId, in.NvMeController.Spec.Id.Value)
	}
	// idempotent API when called with same key, should return same object
	controller, ok := s.Controllers[in.NvMeController.Spec.Id.Value]
	if ok {
		log.Printf("Already existing NVMeController with id %v", in.NvMeController.Spec.Id.Value)
		return controller, nil
	}
	// not found, so create a new one
	subsys, ok := s.Subsystems[in.NvMeController.Spec.SubsystemId.Value]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.NvMeController.Spec.SubsystemId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}

	params := models.MrvlNvmSubsysCreateCtrlrParams{
		Subnqn:       subsys.Spec.Nqn,
		PcieDomainID: int(in.NvMeController.Spec.PcieId.PortId),
		PfID:         int(in.NvMeController.Spec.PcieId.PhysicalFunction),
		VfID:         int(in.NvMeController.Spec.PcieId.VirtualFunction),
		CtrlrID:      int(in.NvMeController.Spec.NvmeControllerId),
		MaxNsq:       int(in.NvMeController.Spec.MaxNsq),
		MaxNcq:       int(in.NvMeController.Spec.MaxNcq),
		Mqes:         int(in.NvMeController.Spec.Sqes),
	}
	var result models.MrvlNvmSubsysCreateCtrlrResult
	err := s.rpc.Call("mrvl_nvm_subsys_create_ctrlr", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		msg := fmt.Sprintf("Could not create CTRL: %s", in.NvMeController.Spec.Id.Value)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	s.Controllers[in.NvMeController.Spec.Id.Value] = in.NvMeController
	s.Controllers[in.NvMeController.Spec.Id.Value].Spec.NvmeControllerId = int32(result.CtrlrID)
	s.Controllers[in.NvMeController.Spec.Id.Value].Status = &pb.NVMeControllerStatus{Active: true}
	response := &pb.NVMeController{Spec: &pb.NVMeControllerSpec{Id: &pc.ObjectKey{Value: "TBD"}}}
	err = deepcopier.Copy(in.NvMeController).To(response)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	return response, nil
}

// DeleteNVMeController deletes an NVMe controller
func (s *Server) DeleteNVMeController(_ context.Context, in *pb.DeleteNVMeControllerRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteNVMeController: Received from client: %v", in)
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
		msg := fmt.Sprintf("Could not delete CTRL: %s", controller.Spec.Id.Value)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	delete(s.Controllers, controller.Spec.Id.Value)
	return &emptypb.Empty{}, nil
}

// UpdateNVMeController updates an NVMe controller
func (s *Server) UpdateNVMeController(_ context.Context, in *pb.UpdateNVMeControllerRequest) (*pb.NVMeController, error) {
	log.Printf("UpdateNVMeController: Received from client: %v", in)
	subsys, ok := s.Subsystems[in.NvMeController.Spec.SubsystemId.Value]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.NvMeController.Spec.SubsystemId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}
	params := models.MrvlNvmSubsysCreateCtrlrParams{
		Subnqn:       subsys.Spec.Nqn,
		PcieDomainID: int(in.NvMeController.Spec.PcieId.PortId),
		PfID:         int(in.NvMeController.Spec.PcieId.PhysicalFunction),
		VfID:         int(in.NvMeController.Spec.PcieId.VirtualFunction),
		CtrlrID:      int(in.NvMeController.Spec.NvmeControllerId),
		MaxNsq:       int(in.NvMeController.Spec.MaxNsq),
		MaxNcq:       int(in.NvMeController.Spec.MaxNcq),
		Mqes:         int(in.NvMeController.Spec.Sqes),
	}
	var result models.MrvlNvmSubsysCreateCtrlrResult
	err := s.rpc.Call("mrvl_nvm_subsys_update_ctrlr", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		msg := fmt.Sprintf("Could not update CTRL: %s", in.NvMeController.Spec.Id.Value)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	s.Controllers[in.NvMeController.Spec.Id.Value] = in.NvMeController
	s.Controllers[in.NvMeController.Spec.Id.Value].Spec.NvmeControllerId = int32(result.CtrlrID)
	s.Controllers[in.NvMeController.Spec.Id.Value].Status = &pb.NVMeControllerStatus{Active: true}
	response := &pb.NVMeController{}
	err = deepcopier.Copy(in.NvMeController).To(response)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	return response, nil
}

// ListNVMeControllers lists NVMe controllers
func (s *Server) ListNVMeControllers(_ context.Context, in *pb.ListNVMeControllersRequest) (*pb.ListNVMeControllersResponse, error) {
	log.Printf("ListNVMeControllers: Received from client: %v", in)
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
	Blobarray := make([]*pb.NVMeController, len(result.CtrlrIDList))
	for i := range result.CtrlrIDList {
		r := &result.CtrlrIDList[i]
		Blobarray[i] = &pb.NVMeController{Spec: &pb.NVMeControllerSpec{NvmeControllerId: int32(r.CtrlrID)}}
	}
	sortNVMeControllers(Blobarray)
	return &pb.ListNVMeControllersResponse{NvMeControllers: Blobarray}, nil
}

// GetNVMeController gets an NVMe controller
func (s *Server) GetNVMeController(_ context.Context, in *pb.GetNVMeControllerRequest) (*pb.NVMeController, error) {
	log.Printf("GetNVMeController: Received from client: %v", in)
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

	return &pb.NVMeController{Spec: &pb.NVMeControllerSpec{Id: &pc.ObjectKey{Value: in.Name}, NvmeControllerId: controller.Spec.NvmeControllerId}, Status: &pb.NVMeControllerStatus{Active: true}}, nil
}

// NVMeControllerStats gets an NVMe controller stats
func (s *Server) NVMeControllerStats(_ context.Context, in *pb.NVMeControllerStatsRequest) (*pb.NVMeControllerStatsResponse, error) {
	log.Printf("NVMeControllerStats: Received from client: %v", in)
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
	return &pb.NVMeControllerStatsResponse{Stats: &pb.VolumeStats{
		ReadBytesCount:    int32(result.NumReadBytes),
		ReadOpsCount:      int32(result.NumReadCmds),
		WriteBytesCount:   int32(result.NumWriteBytes),
		WriteOpsCount:     int32(result.NumWriteCmds),
		ReadLatencyTicks:  int32(result.TotalReadLatencyInUs),
		WriteLatencyTicks: int32(result.TotalWriteLatencyInUs),
	}}, nil
}

// CreateNVMeNamespace creates an NVMe namespace
func (s *Server) CreateNVMeNamespace(_ context.Context, in *pb.CreateNVMeNamespaceRequest) (*pb.NVMeNamespace, error) {
	log.Printf("CreateNVMeNamespace: Received from client: %v", in)
	// see https://google.aip.dev/133#user-specified-ids
	if in.NvMeNamespaceId != "" {
		log.Printf("client provided the ID of a resource %v, ignoring the name field %v", in.NvMeNamespaceId, in.NvMeNamespace.Spec.Id.Value)
	}
	// idempotent API when called with same key, should return same object
	namespace, ok := s.Namespaces[in.NvMeNamespace.Spec.Id.Value]
	if ok {
		log.Printf("Already existing NVMeNamespace with id %v", in.NvMeNamespace.Spec.Id.Value)
		return namespace, nil
	}
	// not found, so create a new one
	subsys, ok := s.Subsystems[in.NvMeNamespace.Spec.SubsystemId.Value]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.NvMeNamespace.Spec.SubsystemId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}
	// TODO: do lookup through VolumeId key instead of using it's value
	params := models.MrvlNvmSubsysAllocNsParams{
		Subnqn:      subsys.Spec.Nqn,
		Nguid:       in.NvMeNamespace.Spec.Nguid,
		Eui64:       strconv.FormatInt(in.NvMeNamespace.Spec.Eui64, 10),
		UUID:        in.NvMeNamespace.Spec.Uuid.Value,
		ShareEnable: 1,
		Bdev:        in.NvMeNamespace.Spec.VolumeId.Value,
	}
	var result models.MrvlNvmSubsysAllocNsResult
	err := s.rpc.Call("mrvl_nvm_subsys_alloc_ns", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		msg := fmt.Sprintf("Could not create NS: %s", in.NvMeNamespace.Spec.Id.Value)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}

	s.Namespaces[in.NvMeNamespace.Spec.Id.Value] = in.NvMeNamespace

	// Now, attach this new NS to ALL controllers
	for _, c := range s.Controllers {
		if c.Spec.SubsystemId.Value != in.NvMeNamespace.Spec.SubsystemId.Value {
			continue
		}
		params := models.MrvlNvmCtrlrAttachNsParams{
			Subnqn:       subsys.Spec.Nqn,
			CtrlrID:      int(c.Spec.NvmeControllerId),
			NsInstanceID: int(in.NvMeNamespace.Spec.HostNsid),
		}
		var result models.MrvlNvmCtrlrAttachNsResult
		err := s.rpc.Call("mrvl_nvm_ctrlr_attach_ns", &params, &result)
		if err != nil {
			log.Printf("error: %v", err)
			return nil, err
		}
		if result.Status != 0 {
			msg := fmt.Sprintf("Could not attach NS: %s", in.NvMeNamespace.Spec.Id.Value)
			log.Print(msg)
			return nil, status.Errorf(codes.InvalidArgument, msg)
		}
	}

	response := &pb.NVMeNamespace{}
	err = deepcopier.Copy(in.NvMeNamespace).To(response)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	response.Status = &pb.NVMeNamespaceStatus{PciState: 2, PciOperState: 1}
	return response, nil
}

// DeleteNVMeNamespace deletes an NVMe namespace
func (s *Server) DeleteNVMeNamespace(_ context.Context, in *pb.DeleteNVMeNamespaceRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteNVMeNamespace: Received from client: %v", in)
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
	delete(s.Namespaces, namespace.Spec.Id.Value)
	return &emptypb.Empty{}, nil
}

// UpdateNVMeNamespace updates an NVMe namespace
func (s *Server) UpdateNVMeNamespace(_ context.Context, in *pb.UpdateNVMeNamespaceRequest) (*pb.NVMeNamespace, error) {
	log.Printf("UpdateNVMeNamespace: Received from client: %v", in)
	return nil, status.Errorf(codes.Unimplemented, "UpdateNVMeNamespace method is not implemented")
}

// ListNVMeNamespaces lists NVMe namespaces
func (s *Server) ListNVMeNamespaces(_ context.Context, in *pb.ListNVMeNamespacesRequest) (*pb.ListNVMeNamespacesResponse, error) {
	log.Printf("ListNVMeNamespaces: Received from client: %v", in)
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
	Blobarray := make([]*pb.NVMeNamespace, len(result.NsList))
	for i := range result.NsList {
		r := &result.NsList[i]
		Blobarray[i] = &pb.NVMeNamespace{Spec: &pb.NVMeNamespaceSpec{HostNsid: int32(r.NsInstanceID)}}
	}
	sortNVMeNamespaces(Blobarray)
	return &pb.ListNVMeNamespacesResponse{NvMeNamespaces: Blobarray}, nil
}

// GetNVMeNamespace gets an NVMe namespace
func (s *Server) GetNVMeNamespace(_ context.Context, in *pb.GetNVMeNamespaceRequest) (*pb.NVMeNamespace, error) {
	log.Printf("GetNVMeNamespace: Received from client: %v", in)
	namespace, ok := s.Namespaces[in.Name]
	if !ok {
		err := status.Errorf(codes.NotFound, "unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}
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
	return &pb.NVMeNamespace{Spec: &pb.NVMeNamespaceSpec{Id: &pc.ObjectKey{Value: in.Name}, Nguid: result.Nguid}, Status: &pb.NVMeNamespaceStatus{PciState: 2, PciOperState: 1}}, nil
}

// NVMeNamespaceStats gets an NVMe namespace stats
func (s *Server) NVMeNamespaceStats(_ context.Context, in *pb.NVMeNamespaceStatsRequest) (*pb.NVMeNamespaceStatsResponse, error) {
	log.Printf("NVMeNamespaceStats: Received from client: %v", in)
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
	return &pb.NVMeNamespaceStatsResponse{Stats: &pb.VolumeStats{
		ReadBytesCount:    int32(result.NumReadBytes),
		ReadOpsCount:      int32(result.NumReadCmds),
		WriteBytesCount:   int32(result.NumWriteBytes),
		WriteOpsCount:     int32(result.NumWriteCmds),
		ReadLatencyTicks:  int32(result.TotalReadLatencyInUs),
		WriteLatencyTicks: int32(result.TotalWriteLatencyInUs),
	}}, nil
}
