// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Dell Inc, or its subsidiaries.
// Copyright (C) 2022 Marvell International Ltd.

package main

import (
	"context"
	"fmt"
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

// ////////////////////////////////////////////////////////
var subsystems = map[string]*pb.NVMeSubsystem{}

func (s *server) CreateNVMeSubsystem(ctx context.Context, in *pb.CreateNVMeSubsystemRequest) (*pb.NVMeSubsystem, error) {
	log.Printf("CreateNVMeSubsystem: Received from client: %v", in)
	// TODO: fix const values below
	params := MrvlNvmCreateSubsystemParams{
		Subnqn:        in.Subsystem.Spec.Nqn,
		Mn:            in.Subsystem.Spec.ModelNumber,
		Sn:            in.Subsystem.Spec.SerialNumber,
		MaxNamespaces: int(in.Subsystem.Spec.MaxNamespaces),
		MinCtrlrID:    3,
		MaxCtrlrID:    256,
	}
	var result MrvlNvmCreateSubsystemResult
	err := call("mrvl_nvm_create_subsystem", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	subsystems[in.Subsystem.Spec.Id.Value] = in.Subsystem
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		log.Printf("Could not create: %v", in)
	}
	response := &pb.NVMeSubsystem{}
	err = deepcopier.Copy(in.Subsystem).To(response)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	return response, nil
}

func (s *server) DeleteNVMeSubsystem(ctx context.Context, in *pb.DeleteNVMeSubsystemRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteNVMeSubsystem: Received from client: %v", in)
	subsys, ok := subsystems[in.SubsystemId.Value]
	if !ok {
		err := fmt.Errorf("unable to find key %s", in.SubsystemId)
		log.Printf("error: %v", err)
		return nil, err
	}
	params := MrvlNvmDeleteSubsystemParams{
		Subnqn: subsys.Spec.Nqn,
	}
	var result MrvlNvmDeleteSubsystemResult
	err := call("mrvl_nvm_deletesubsystem", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		log.Printf("Could not delete: %v", in)
	}
	delete(subsystems, subsys.Spec.Id.Value)
	return &emptypb.Empty{}, nil
}

func (s *server) UpdateNVMeSubsystem(ctx context.Context, in *pb.UpdateNVMeSubsystemRequest) (*pb.NVMeSubsystem, error) {
	log.Printf("UpdateNVMeSubsystem: Received from client: %v", in)
	return nil, status.Errorf(codes.Unimplemented, "UpdateNVMeSubsystem method is not implemented")
}

func (s *server) ListNVMeSubsystem(ctx context.Context, in *pb.ListNVMeSubsystemRequest) (*pb.ListNVMeSubsystemResponse, error) {
	log.Printf("ListNVMeSubsystem: Received from client: %v", in)
	var result MrvvNvmGetSubsysListResult
	err := call("mrvl_nvm_get_subsys_list", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		log.Printf("Could not create: %v", in)
	}

	Blobarray := make([]*pb.NVMeSubsystem, len(result.SubsysList))
	for i := range result.SubsysList {
		r := &result.SubsysList[i]
		Blobarray[i] = &pb.NVMeSubsystem{Spec: &pb.NVMeSubsystemSpec{Nqn: r.Subnqn}}
	}
	return &pb.ListNVMeSubsystemResponse{Subsystems: Blobarray}, nil
}

func (s *server) GetNVMeSubsystem(ctx context.Context, in *pb.GetNVMeSubsystemRequest) (*pb.NVMeSubsystem, error) {
	log.Printf("GetNVMeSubsystem: Received from client: %v", in)
	subsys, ok := subsystems[in.SubsystemId.Value]
	if !ok {
		err := fmt.Errorf("unable to find key %s", in.SubsystemId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}
	// TODO: replace with MRVL code : mrvl_nvm_subsys_get_info ?
	var result MrvvNvmGetSubsysListResult
	err := call("mrvl_nvm_get_subsys_list", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		log.Printf("Could not get: %v", in)
	}
	for i := range result.SubsysList {
		r := &result.SubsysList[i]
		if r.Subnqn == subsys.Spec.Nqn {
			return &pb.NVMeSubsystem{Spec: &pb.NVMeSubsystemSpec{Nqn: r.Subnqn}}, nil
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
		err := fmt.Errorf("unable to find key %s", in.SubsystemId)
		log.Printf("error: %v", err)
		return nil, err
	}
	params := MrvlNvmGetSubsysInfoParams{
		Subnqn: subsys.Spec.Nqn,
	}
	var result MrvlNvmGetSubsysInfoResult
	err := call("mrvl_nvm_subsys_get_info", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		log.Printf("Could not get stats: %v", in)
	}
	return &pb.NVMeSubsystemStatsResponse{Stats: "TBD"}, nil
}

// ////////////////////////////////////////////////////////
var controllers = map[string]*pb.NVMeController{}

func (s *server) CreateNVMeController(ctx context.Context, in *pb.CreateNVMeControllerRequest) (*pb.NVMeController, error) {
	log.Printf("CreateNVMeController: Received from client: %v", in)
	subsys, ok := subsystems[in.Controller.Spec.SubsystemId.Value]
	if !ok {
		err := fmt.Errorf("unable to find key %s", in.Controller.Spec.SubsystemId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}

	params := MrvlNvmSubsysCreateCtrlrParams{
		Subnqn:       subsys.Spec.Nqn,
		PcieDomainID: int(in.Controller.Spec.PcieId.PortId),
		IsPf:         int(in.Controller.Spec.PcieId.PhysicalFunction),
		InstanceID:   int(in.Controller.Spec.PcieId.VirtualFunction),
		MaxNsq:       int(in.Controller.Spec.MaxNsq),
		MaxNcq:       int(in.Controller.Spec.MaxNcq),
	}
	var result MrvlNvmSubsysCreateCtrlrResult
	err := call("mrvl_nvm_subsys_create_ctrlr", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		log.Printf("Could not get stats: %v", in)
	}
	controllers[in.Controller.Spec.Id.Value] = in.Controller
	controllers[in.Controller.Spec.Id.Value].Spec.NvmeControllerId = int32(result.CtrlrID)
	response := &pb.NVMeController{Spec: &pb.NVMeControllerSpec{Id: &pc.ObjectKey{Value: "TBD"}}}
	err = deepcopier.Copy(in.Controller).To(response)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	return response, nil
}

func (s *server) DeleteNVMeController(ctx context.Context, in *pb.DeleteNVMeControllerRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteNVMeController: Received from client: %v", in)
	controller, ok := controllers[in.ControllerId.Value]
	if !ok {
		return nil, fmt.Errorf("error finding controller %s", in.ControllerId.Value)
	}
	subsys, ok := subsystems[controller.Spec.SubsystemId.Value]
	if !ok {
		err := fmt.Errorf("unable to find key %s", controller.Spec.SubsystemId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}

	params := MrvlNvmSubsysRemoveCtrlrParams{
		Subnqn:  subsys.Spec.Nqn,
		CntlrID: int(controller.Spec.NvmeControllerId),
	}
	var result MrvlNvmSubsysRemoveCtrlrResult
	err := call("mrvl_nvm_subsys_remove_ctrlr", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		log.Printf("Could not delete: %v", in)
	}
	delete(controllers, controller.Spec.Id.Value)
	return &emptypb.Empty{}, nil
}

func (s *server) UpdateNVMeController(ctx context.Context, in *pb.UpdateNVMeControllerRequest) (*pb.NVMeController, error) {
	log.Printf("UpdateNVMeController: Received from client: %v", in)
	subsys, ok := subsystems[in.Controller.Spec.SubsystemId.Value]
	if !ok {
		err := fmt.Errorf("unable to find key %s", in.Controller.Spec.SubsystemId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}
	params := MrvlNvmSubsysCreateCtrlrParams{
		Subnqn:       subsys.Spec.Nqn,
		PcieDomainID: int(in.Controller.Spec.PcieId.PortId),
		IsPf:         int(in.Controller.Spec.PcieId.PhysicalFunction),
		InstanceID:   int(in.Controller.Spec.PcieId.VirtualFunction),
		MaxNsq:       int(in.Controller.Spec.MaxNsq),
		MaxNcq:       int(in.Controller.Spec.MaxNcq),
	}
	var result MrvlNvmSubsysCreateCtrlrResult
	err := call("mrvl_nvm_subsys_update_ctrlr", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		log.Printf("Could not delete: %v", in)
	}
	controllers[in.Controller.Spec.Id.Value] = in.Controller
	response := &pb.NVMeController{}
	err = deepcopier.Copy(in.Controller).To(response)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	return response, nil
}

func (s *server) ListNVMeController(ctx context.Context, in *pb.ListNVMeControllerRequest) (*pb.ListNVMeControllerResponse, error) {
	log.Printf("ListNVMeController: Received from client: %v", in)
	var result MrvlNvmSubsysGetCtrlrListResult
	err := call("mrvl_nvm_subsys_get_ctrlr_list", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		log.Printf("Could not delete: %v", in)
	}
	Blobarray := make([]*pb.NVMeController, len(result.CtrlrIDList))
	for i := range result.CtrlrIDList {
		r := &result.CtrlrIDList[i]
		Blobarray[i] = &pb.NVMeController{Spec: &pb.NVMeControllerSpec{NvmeControllerId: int32(r.CtrlrID)}}
	}
	return &pb.ListNVMeControllerResponse{Controllers: Blobarray}, nil
}

func (s *server) GetNVMeController(ctx context.Context, in *pb.GetNVMeControllerRequest) (*pb.NVMeController, error) {
	log.Printf("GetNVMeController: Received from client: %v", in)
	controller, ok := controllers[in.ControllerId.Value]
	if !ok {
		return nil, fmt.Errorf("error finding controller %s", in.ControllerId.Value)
	}
	subsys, ok := subsystems[controller.Spec.SubsystemId.Value]
	if !ok {
		err := fmt.Errorf("unable to find key %s", controller.Spec.SubsystemId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}

	params := MrvlNvmGetCtrlrInfoParams{
		Subnqn:  subsys.Spec.Nqn,
		CtrlrID: int(controller.Spec.NvmeControllerId),
	}
	var result MrvlNvmGetCtrlrInfoResult
	err := call("mrvl_nvm_ctrlr_get_info", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		log.Printf("Could not get stats: %v", in)
	}

	return &pb.NVMeController{Spec: &pb.NVMeControllerSpec{Id: in.ControllerId, NvmeControllerId: controller.Spec.NvmeControllerId}}, nil
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

	params := MrvlNvmGetCtrlrStatsParams{
		Subnqn: subsys.Spec.Nqn,
	}
	var result MrvlNvmGetCtrlrStatsResult
	err := call("mrvl_nvm_ctrlr_get_stats", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		log.Printf("Could not get stats: %v", in)
	}
	return &pb.NVMeControllerStatsResponse{Stats: "TBD"}, nil
}

// ////////////////////////////////////////////////////////
var namespaces = map[string]*pb.NVMeNamespace{}

func (s *server) CreateNVMeNamespace(ctx context.Context, in *pb.CreateNVMeNamespaceRequest) (*pb.NVMeNamespace, error) {
	log.Printf("CreateNVMeNamespace: Received from client: %v", in)
	subsys, ok := subsystems[in.Namespace.Spec.SubsystemId.Value]
	if !ok {
		err := fmt.Errorf("unable to find key %s", in.Namespace.Spec.SubsystemId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}
	// TODO: do lookup through VolumeId key instead of using it's value
	params := MrvlNvmSubsysAllocNsParams{
		Subnqn:       subsys.Spec.Nqn,
		Nguid:        in.Namespace.Spec.Nguid,
		Eui64:        strconv.FormatInt(in.Namespace.Spec.Eui64, 10),
		UUID:         in.Namespace.Spec.Uuid.Value,
		NsInstanceID: int(in.Namespace.Spec.HostNsid),
		ShareEnable:  0,
		Bdev:         in.Namespace.Spec.VolumeId.Value,
	}
	var result MrvlNvmSubsysAllocNsResult
	err := call("mrvl_nvm_subsys_alloc_ns", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	namespaces[in.Namespace.Spec.Id.Value] = in.Namespace

	// Now, attach this new NS to ALL controllers
	for _, c := range controllers {
		if c.Spec.SubsystemId.Value != in.Namespace.Spec.SubsystemId.Value {
			continue
		}
		params := MrvlNvmCtrlrAttachNsParams{
			Subnqn:       subsys.Spec.Nqn,
			CtrlID:       int(c.Spec.NvmeControllerId),
			NsInstanceID: int(in.Namespace.Spec.HostNsid),
		}
		var result MrvlNvmCtrlrAttachNsResult
		err := call("mrvl_nvm_ctrlr_attach_ns", &params, &result)
		if err != nil {
			log.Printf("error: %v", err)
			return nil, err
		}
		if result.Status != 0 {
			log.Printf("Could not get stats: %v", in)
		}
	}

	response := &pb.NVMeNamespace{}
	err = deepcopier.Copy(in.Namespace).To(response)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	return response, nil
}

func (s *server) DeleteNVMeNamespace(ctx context.Context, in *pb.DeleteNVMeNamespaceRequest) (*emptypb.Empty, error) {
	log.Printf("DeleteNVMeNamespace: Received from client: %v", in)
	namespace, ok := namespaces[in.NamespaceId.Value]
	if !ok {
		err := fmt.Errorf("unable to find key %s", in.NamespaceId.Value)
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
		params := MrvlNvmCtrlrDetachNsParams{
			Subnqn:       subsys.Spec.Nqn,
			CtrlrID:      int(c.Spec.NvmeControllerId),
			NsInstanceID: int(namespace.Spec.HostNsid),
		}
		var result MrvlNvmCtrlrDetachNsResult
		err := call("mrvl_nvm_ctrlr_detach_ns", &params, &result)
		if err != nil {
			log.Printf("error: %v", err)
			return nil, err
		}
		if result.Status != 0 {
			log.Printf("Could not get stats: %v", in)
		}
	}
	params := MrvlNvmSubsysUnallocNsParams{
		Subnqn:       subsys.Spec.Nqn,
		NsInstanceID: int(namespace.Spec.HostNsid),
	}
	var result MrvlNvmSubsysUnallocNsResult
	err := call(" mrvl_nvm_subsys_unalloc_ns", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	delete(namespaces, namespace.Spec.Id.Value)
	return &emptypb.Empty{}, nil
}

func (s *server) UpdateNVMeNamespace(ctx context.Context, in *pb.UpdateNVMeNamespaceRequest) (*pb.NVMeNamespace, error) {
	log.Printf("UpdateNVMeNamespace: Received from client: %v", in)
	return nil, status.Errorf(codes.Unimplemented, "UpdateNVMeNamespace method is not implemented")
}

func (s *server) ListNVMeNamespace(ctx context.Context, in *pb.ListNVMeNamespaceRequest) (*pb.ListNVMeNamespaceResponse, error) {
	log.Printf("ListNVMeNamespace: Received from client: %v", in)
	subsys, ok := subsystems[in.SubsystemId.Value]
	if !ok {
		err := fmt.Errorf("unable to find key %s", in.SubsystemId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}
	params := MrvlNvmSubsysGetNsListParams{
		Subnqn: subsys.Spec.Nqn,
	}
	var result MrvlNvmSubsysGetNsListResult
	err := call("mrvl_nvm_subsys_get_ns_list", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		log.Printf("Could not delete: %v", in)
	}
	Blobarray := make([]*pb.NVMeNamespace, len(result.NsList))
	for i := range result.NsList {
		r := &result.NsList[i]
		Blobarray[i] = &pb.NVMeNamespace{Spec: &pb.NVMeNamespaceSpec{HostNsid: int32(r.NsInstanceID)}}
	}
	return &pb.ListNVMeNamespaceResponse{Namespaces: Blobarray}, nil
}

func (s *server) GetNVMeNamespace(ctx context.Context, in *pb.GetNVMeNamespaceRequest) (*pb.NVMeNamespace, error) {
	log.Printf("GetNVMeNamespace: Received from client: %v", in)
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

	params := MrvlNvmGetNsInfoParams{
		SubNqn:       subsys.Spec.Nqn,
		NsInstanceID: int(namespace.Spec.HostNsid),
	}
	var result MrvlNvmGetNsInfoResult
	err := call("mrvl_nvm_ns_get_info", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		log.Printf("Could not get stats: %v", in)
	}
	return &pb.NVMeNamespace{Spec: &pb.NVMeNamespaceSpec{Id: in.NamespaceId, Nguid: result.Nguid}}, nil
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

	params := MrvlNvmGetNsStatsParams{
		SubNqn:       subsys.Spec.Nqn,
		NsInstanceID: int(namespace.Spec.HostNsid),
	}
	var result MrvlNvmGetNsStatsResult
	err := call("mrvl_nvm_ns_get_stats", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if result.Status != 0 {
		log.Printf("Could not get stats: %v", in)
	}
	return &pb.NVMeNamespaceStatsResponse{Stats: "TBD"}, nil
}
