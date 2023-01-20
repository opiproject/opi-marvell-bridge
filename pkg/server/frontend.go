// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Dell Inc, or its subsidiaries.
// Copyright (C) 2022 Marvell International Ltd.

package server

import (
	"context"
	"fmt"
	"github.com/opiproject/opi-marvell-bridge/pkg/jsonrpc"
	"github.com/opiproject/opi-marvell-bridge/pkg/spdk"
	"log"
	"strconv"

	pc "github.com/opiproject/opi-api/common/v1/gen/go"
	pb "github.com/opiproject/opi-api/storage/v1alpha1/gen/go"
	"github.com/ulule/deepcopier"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type server struct {
	pb.UnimplementedFrontendNvmeServiceServer
}

var PluginFrontendNvme server
var subsystems = map[string]*pb.NVMeSubsystem{}

func (s *server) CreateNVMeSubsystem(ctx context.Context, in *pb.CreateNVMeSubsystemRequest) (*pb.NVMeSubsystem, error) {
	log.Printf("CreateNVMeSubsystem: Received from client: %v", in)
	// TODO: fix const values below
	params := spdk.MrvlNvmCreateSubsystemParams{
		Subnqn:        in.NvMeSubsystem.Spec.Nqn,
		Mn:            in.NvMeSubsystem.Spec.ModelNumber,
		Sn:            in.NvMeSubsystem.Spec.SerialNumber,
		MaxNamespaces: int(in.NvMeSubsystem.Spec.MaxNamespaces),
		MinCtrlrID:    3,
		MaxCtrlrID:    256,
	}
	var result spdk.MrvlNvmCreateSubsystemResult
	err := jsonrpc.Call("mrvl_nvm_create_subsystem", &params, &result)
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
	var ver spdk.MrvlNvmGetOffloadCapResult
	err = jsonrpc.Call("mrvl_nvm_get_offload_cap", nil, &ver)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", ver)
	if ver.Status != 0 {
		msg := fmt.Sprintf("Could not get FW version for NQN create request: %s", in.NvMeSubsystem.Spec.Nqn)
		log.Print(msg)
		return nil, status.Errorf(codes.InvalidArgument, msg)
	}
	response := &pb.NVMeSubsystem{}
	err = deepcopier.Copy(in.NvMeSubsystem).To(response)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	response.Status = &pb.NVMeSubsystemStatus{FirmwareRevision: ver.SdkVersion}
	subsystems[in.NvMeSubsystem.Spec.Id.Value] = response
	return response, nil
}

func (s *server) DeleteNVMeSubsystem(ctx context.Context, in *pb.DeleteNVMeSubsystemRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteNVMeSubsystem: Received from client: %v", in)
	subsys, ok := subsystems[in.Name]
	if !ok {
		err := fmt.Errorf("unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}
	params := spdk.MrvlNvmDeleteSubsystemParams{
		Subnqn: subsys.Spec.Nqn,
	}
	var result spdk.MrvlNvmDeleteSubsystemResult
	err := jsonrpc.Call("mrvl_nvm_deletesubsystem", &params, &result)
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
	delete(subsystems, subsys.Spec.Id.Value)
	return &emptypb.Empty{}, nil
}

func (s *server) UpdateNVMeSubsystem(ctx context.Context, in *pb.UpdateNVMeSubsystemRequest) (*pb.NVMeSubsystem, error) {
	log.Printf("UpdateNVMeSubsystem: Received from client: %v", in)
	return nil, status.Errorf(codes.Unimplemented, "UpdateNVMeSubsystem method is not implemented")
}

func (s *server) ListNVMeSubsystems(ctx context.Context, in *pb.ListNVMeSubsystemsRequest) (*pb.ListNVMeSubsystemsResponse, error) {
	log.Printf("ListNVMeSubsystems: Received from client: %v", in)
	var result spdk.MrvvNvmGetSubsysListResult
	err := jsonrpc.Call("mrvl_nvm_get_subsys_list", nil, &result)
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

	Blobarray := make([]*pb.NVMeSubsystem, len(result.SubsysList))
	for i := range result.SubsysList {
		r := &result.SubsysList[i]
		Blobarray[i] = &pb.NVMeSubsystem{Spec: &pb.NVMeSubsystemSpec{Nqn: r.Subnqn}}
	}
	return &pb.ListNVMeSubsystemsResponse{NvMeSubsystems: Blobarray}, nil
}

func (s *server) GetNVMeSubsystem(ctx context.Context, in *pb.GetNVMeSubsystemRequest) (*pb.NVMeSubsystem, error) {
	log.Printf("GetNVMeSubsystem: Received from client: %v", in)
	subsys, ok := subsystems[in.Name]
	if !ok {
		err := fmt.Errorf("unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}
	// TODO: replace with MRVL code : mrvl_nvm_subsys_get_info ?
	var result spdk.MrvvNvmGetSubsysListResult
	err := jsonrpc.Call("mrvl_nvm_get_subsys_list", nil, &result)
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

func (s *server) NVMeSubsystemStats(ctx context.Context, in *pb.NVMeSubsystemStatsRequest) (*pb.NVMeSubsystemStatsResponse, error) {
	log.Printf("NVMeSubsystemStats: Received from client: %v", in)
	subsys, ok := subsystems[in.SubsystemId.Value]
	if !ok {
		err := fmt.Errorf("unable to find key %s", in.SubsystemId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}
	params := spdk.MrvlNvmGetSubsysInfoParams{
		Subnqn: subsys.Spec.Nqn,
	}
	var result spdk.MrvlNvmGetSubsysInfoResult
	err := jsonrpc.Call("mrvl_nvm_subsys_get_info", &params, &result)
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

// ////////////////////////////////////////////////////////
var controllers = map[string]*pb.NVMeController{}

func (s *server) CreateNVMeController(ctx context.Context, in *pb.CreateNVMeControllerRequest) (*pb.NVMeController, error) {
	log.Printf("CreateNVMeController: Received from client: %v", in)
	subsys, ok := subsystems[in.NvMeController.Spec.SubsystemId.Value]
	if !ok {
		err := fmt.Errorf("unable to find key %s", in.NvMeController.Spec.SubsystemId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}

	params := spdk.MrvlNvmSubsysCreateCtrlrParams{
		Subnqn:       subsys.Spec.Nqn,
		PcieDomainID: int(in.NvMeController.Spec.PcieId.PortId),
		PfID:         int(in.NvMeController.Spec.PcieId.PhysicalFunction),
		VfID:         int(in.NvMeController.Spec.PcieId.VirtualFunction),
		CtrlID:       int(in.NvMeController.Spec.NvmeControllerId),
		MaxNsq:       int(in.NvMeController.Spec.MaxNsq),
		MaxNcq:       int(in.NvMeController.Spec.MaxNcq),
		Sqes:         int(in.NvMeController.Spec.Sqes),
		Cqes:         int(in.NvMeController.Spec.Cqes),
	}
	var result spdk.MrvlNvmSubsysCreateCtrlrResult
	err := jsonrpc.Call("mrvl_nvm_subsys_create_ctrlr", &params, &result)
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
	controllers[in.NvMeController.Spec.Id.Value] = in.NvMeController
	controllers[in.NvMeController.Spec.Id.Value].Spec.NvmeControllerId = int32(result.CtrlrID)
	controllers[in.NvMeController.Spec.Id.Value].Status = &pb.NVMeControllerStatus{Active: true}
	response := &pb.NVMeController{Spec: &pb.NVMeControllerSpec{Id: &pc.ObjectKey{Value: "TBD"}}}
	err = deepcopier.Copy(in.NvMeController).To(response)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	return response, nil
}

func (s *server) DeleteNVMeController(ctx context.Context, in *pb.DeleteNVMeControllerRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteNVMeController: Received from client: %v", in)
	controller, ok := controllers[in.Name]
	if !ok {
		return nil, fmt.Errorf("error finding controller %s", in.Name)
	}
	subsys, ok := subsystems[controller.Spec.SubsystemId.Value]
	if !ok {
		err := fmt.Errorf("unable to find key %s", controller.Spec.SubsystemId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}

	params := spdk.MrvlNvmSubsysRemoveCtrlrParams{
		Subnqn:  subsys.Spec.Nqn,
		CntlrID: int(controller.Spec.NvmeControllerId),
	}
	var result spdk.MrvlNvmSubsysRemoveCtrlrResult
	err := jsonrpc.Call("mrvl_nvm_subsys_remove_ctrlr", &params, &result)
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
	delete(controllers, controller.Spec.Id.Value)
	return &emptypb.Empty{}, nil
}

func (s *server) UpdateNVMeController(ctx context.Context, in *pb.UpdateNVMeControllerRequest) (*pb.NVMeController, error) {
	log.Printf("UpdateNVMeController: Received from client: %v", in)
	subsys, ok := subsystems[in.NvMeController.Spec.SubsystemId.Value]
	if !ok {
		err := fmt.Errorf("unable to find key %s", in.NvMeController.Spec.SubsystemId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}
	params := spdk.MrvlNvmSubsysCreateCtrlrParams{
		Subnqn:       subsys.Spec.Nqn,
		PcieDomainID: int(in.NvMeController.Spec.PcieId.PortId),
		PfID:         int(in.NvMeController.Spec.PcieId.PhysicalFunction),
		VfID:         int(in.NvMeController.Spec.PcieId.VirtualFunction),
		CtrlID:       int(in.NvMeController.Spec.NvmeControllerId),
		MaxNsq:       int(in.NvMeController.Spec.MaxNsq),
		MaxNcq:       int(in.NvMeController.Spec.MaxNcq),
		Sqes:         int(in.NvMeController.Spec.Sqes),
		Cqes:         int(in.NvMeController.Spec.Cqes),
	}
	var result spdk.MrvlNvmSubsysCreateCtrlrResult
	err := jsonrpc.Call("mrvl_nvm_subsys_update_ctrlr", &params, &result)
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
	controllers[in.NvMeController.Spec.Id.Value] = in.NvMeController
	controllers[in.NvMeController.Spec.Id.Value].Spec.NvmeControllerId = int32(result.CtrlrID)
	controllers[in.NvMeController.Spec.Id.Value].Status = &pb.NVMeControllerStatus{Active: true}
	response := &pb.NVMeController{}
	err = deepcopier.Copy(in.NvMeController).To(response)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	return response, nil
}

func (s *server) ListNVMeControllers(ctx context.Context, in *pb.ListNVMeControllersRequest) (*pb.ListNVMeControllersResponse, error) {
	log.Printf("ListNVMeControllers: Received from client: %v", in)
	// TODO: missing MrvlNvmSubsysGetCtrlrListParams
	var result spdk.MrvlNvmSubsysGetCtrlrListResult
	err := jsonrpc.Call("mrvl_nvm_subsys_get_ctrlr_list", nil, &result)
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
	Blobarray := make([]*pb.NVMeController, len(result.CtrlrIDList))
	for i := range result.CtrlrIDList {
		r := &result.CtrlrIDList[i]
		Blobarray[i] = &pb.NVMeController{Spec: &pb.NVMeControllerSpec{NvmeControllerId: int32(r.CtrlrID)}}
	}
	return &pb.ListNVMeControllersResponse{NvMeControllers: Blobarray}, nil
}

func (s *server) GetNVMeController(ctx context.Context, in *pb.GetNVMeControllerRequest) (*pb.NVMeController, error) {
	log.Printf("GetNVMeController: Received from client: %v", in)
	controller, ok := controllers[in.Name]
	if !ok {
		return nil, fmt.Errorf("error finding controller %s", in.Name)
	}
	subsys, ok := subsystems[controller.Spec.SubsystemId.Value]
	if !ok {
		err := fmt.Errorf("unable to find key %s", controller.Spec.SubsystemId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}

	params := spdk.MrvlNvmGetCtrlrInfoParams{
		Subnqn:  subsys.Spec.Nqn,
		CtrlrID: int(controller.Spec.NvmeControllerId),
	}
	var result spdk.MrvlNvmGetCtrlrInfoResult
	err := jsonrpc.Call("mrvl_nvm_ctrlr_get_info", &params, &result)
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

func (s *server) NVMeControllerStats(ctx context.Context, in *pb.NVMeControllerStatsRequest) (*pb.NVMeControllerStatsResponse, error) {
	log.Printf("NVMeControllerStats: Received from client: %v", in)
	controller, ok := controllers[in.Id.Value]
	if !ok {
		return nil, fmt.Errorf("error finding controller %s", in.Id.Value)
	}
	subsys, ok := subsystems[controller.Spec.SubsystemId.Value]
	if !ok {
		err := fmt.Errorf("unable to find key %s", controller.Spec.SubsystemId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}

	params := spdk.MrvlNvmGetCtrlrStatsParams{
		Subnqn: subsys.Spec.Nqn,
	}
	var result spdk.MrvlNvmGetCtrlrStatsResult
	err := jsonrpc.Call("mrvl_nvm_ctrlr_get_stats", &params, &result)
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

// ////////////////////////////////////////////////////////
var namespaces = map[string]*pb.NVMeNamespace{}

func (s *server) CreateNVMeNamespace(ctx context.Context, in *pb.CreateNVMeNamespaceRequest) (*pb.NVMeNamespace, error) {
	log.Printf("CreateNVMeNamespace: Received from client: %v", in)
	subsys, ok := subsystems[in.NvMeNamespace.Spec.SubsystemId.Value]
	if !ok {
		err := fmt.Errorf("unable to find key %s", in.NvMeNamespace.Spec.SubsystemId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}
	// TODO: do lookup through VolumeId key instead of using it's value
	params := spdk.MrvlNvmSubsysAllocNsParams{
		Subnqn:       subsys.Spec.Nqn,
		Nguid:        in.NvMeNamespace.Spec.Nguid,
		Eui64:        strconv.FormatInt(in.NvMeNamespace.Spec.Eui64, 10),
		UUID:         in.NvMeNamespace.Spec.Uuid.Value,
		NsInstanceID: int(in.NvMeNamespace.Spec.HostNsid),
		ShareEnable:  0,
		Bdev:         in.NvMeNamespace.Spec.VolumeId.Value,
	}
	var result spdk.MrvlNvmSubsysAllocNsResult
	err := jsonrpc.Call("mrvl_nvm_subsys_alloc_ns", &params, &result)
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

	namespaces[in.NvMeNamespace.Spec.Id.Value] = in.NvMeNamespace

	// Now, attach this new NS to ALL controllers
	for _, c := range controllers {
		if c.Spec.SubsystemId.Value != in.NvMeNamespace.Spec.SubsystemId.Value {
			continue
		}
		params := spdk.MrvlNvmCtrlrAttachNsParams{
			Subnqn:       subsys.Spec.Nqn,
			CtrlID:       int(c.Spec.NvmeControllerId),
			NsInstanceID: int(in.NvMeNamespace.Spec.HostNsid),
		}
		var result spdk.MrvlNvmCtrlrAttachNsResult
		err := jsonrpc.Call("mrvl_nvm_ctrlr_attach_ns", &params, &result)
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

func (s *server) DeleteNVMeNamespace(ctx context.Context, in *pb.DeleteNVMeNamespaceRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteNVMeNamespace: Received from client: %v", in)
	namespace, ok := namespaces[in.Name]
	if !ok {
		err := fmt.Errorf("unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}
	subsys, ok := subsystems[namespace.Spec.SubsystemId.Value]
	if !ok {
		err := fmt.Errorf("unable to find subsystem %s", namespace.Spec.SubsystemId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}
	// First, detach this NS from ALL controllers
	for _, c := range controllers {
		if c.Spec.SubsystemId.Value != namespace.Spec.SubsystemId.Value {
			continue
		}
		params := spdk.MrvlNvmCtrlrDetachNsParams{
			Subnqn:       subsys.Spec.Nqn,
			CtrlrID:      int(c.Spec.NvmeControllerId),
			NsInstanceID: int(namespace.Spec.HostNsid),
		}
		var result spdk.MrvlNvmCtrlrDetachNsResult
		err := jsonrpc.Call("mrvl_nvm_ctrlr_detach_ns", &params, &result)
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
	params := spdk.MrvlNvmSubsysUnallocNsParams{
		Subnqn:       subsys.Spec.Nqn,
		NsInstanceID: int(namespace.Spec.HostNsid),
	}
	var result spdk.MrvlNvmSubsysUnallocNsResult
	err := jsonrpc.Call(" mrvl_nvm_subsys_unalloc_ns", &params, &result)
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
	delete(namespaces, namespace.Spec.Id.Value)
	return &emptypb.Empty{}, nil
}

func (s *server) UpdateNVMeNamespace(ctx context.Context, in *pb.UpdateNVMeNamespaceRequest) (*pb.NVMeNamespace, error) {
	log.Printf("UpdateNVMeNamespace: Received from client: %v", in)
	return nil, status.Errorf(codes.Unimplemented, "UpdateNVMeNamespace method is not implemented")
}

func (s *server) ListNVMeNamespaces(ctx context.Context, in *pb.ListNVMeNamespacesRequest) (*pb.ListNVMeNamespacesResponse, error) {
	log.Printf("ListNVMeNamespaces: Received from client: %v", in)
	subsys, ok := subsystems[in.Parent]
	if !ok {
		err := fmt.Errorf("unable to find key %s", in.Parent)
		log.Printf("error: %v", err)
		return nil, err
	}
	params := spdk.MrvlNvmSubsysGetNsListParams{
		Subnqn: subsys.Spec.Nqn,
	}
	var result spdk.MrvlNvmSubsysGetNsListResult
	err := jsonrpc.Call("mrvl_nvm_subsys_get_ns_list", &params, &result)
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
	Blobarray := make([]*pb.NVMeNamespace, len(result.NsList))
	for i := range result.NsList {
		r := &result.NsList[i]
		Blobarray[i] = &pb.NVMeNamespace{Spec: &pb.NVMeNamespaceSpec{HostNsid: int32(r.NsInstanceID)}}
	}
	return &pb.ListNVMeNamespacesResponse{NvMeNamespaces: Blobarray}, nil
}

func (s *server) GetNVMeNamespace(ctx context.Context, in *pb.GetNVMeNamespaceRequest) (*pb.NVMeNamespace, error) {
	log.Printf("GetNVMeNamespace: Received from client: %v", in)
	namespace, ok := namespaces[in.Name]
	if !ok {
		err := fmt.Errorf("unable to find key %s", in.Name)
		log.Printf("error: %v", err)
		return nil, err
	}
	subsys, ok := subsystems[namespace.Spec.SubsystemId.Value]
	if !ok {
		err := fmt.Errorf("unable to find key %s", namespace.Spec.SubsystemId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}

	params := spdk.MrvlNvmGetNsInfoParams{
		SubNqn:       subsys.Spec.Nqn,
		NsInstanceID: int(namespace.Spec.HostNsid),
	}
	var result spdk.MrvlNvmGetNsInfoResult
	err := jsonrpc.Call("mrvl_nvm_ns_get_info", &params, &result)
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

func (s *server) NVMeNamespaceStats(ctx context.Context, in *pb.NVMeNamespaceStatsRequest) (*pb.NVMeNamespaceStatsResponse, error) {
	log.Printf("NVMeNamespaceStats: Received from client: %v", in)
	namespace, ok := namespaces[in.NamespaceId.Value]
	if !ok {
		err := fmt.Errorf("unable to find key %s", in.NamespaceId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}
	subsys, ok := subsystems[namespace.Spec.SubsystemId.Value]
	if !ok {
		err := fmt.Errorf("unable to find key %s", namespace.Spec.SubsystemId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}

	params := spdk.MrvlNvmGetNsStatsParams{
		SubNqn:       subsys.Spec.Nqn,
		NsInstanceID: int(namespace.Spec.HostNsid),
	}
	var result spdk.MrvlNvmGetNsStatsResult
	err := jsonrpc.Call("mrvl_nvm_ns_get_stats", &params, &result)
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
