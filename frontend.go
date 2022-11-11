// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Dell Inc, or its subsidiaries.
// Copyright (C) 2022 Marvell International Ltd.

package main

import (
	"context"
	"fmt"
	"log"

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

func (s *server) NVMeSubsystemCreate(ctx context.Context, in *pb.NVMeSubsystemCreateRequest) (*pb.NVMeSubsystem, error) {
	log.Printf("NVMeSubsystemCreate: Received from client: %v", in)
	params := MrvlNvmCreateSubsystemParams{
		Subnqn: in.Subsystem.Spec.Nqn,
		Mn:     "OpiModel0",
		Sn:     "OpiSerial0",
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

func (s *server) NVMeSubsystemDelete(ctx context.Context, in *pb.NVMeSubsystemDeleteRequest) (*emptypb.Empty, error) {
	log.Printf("NVMeSubsystemDelete: Received from client: %v", in)
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

func (s *server) NVMeSubsystemUpdate(ctx context.Context, in *pb.NVMeSubsystemUpdateRequest) (*pb.NVMeSubsystem, error) {
	log.Printf("NVMeSubsystemUpdate: Received from client: %v", in)
	return nil, status.Errorf(codes.Unimplemented, "NVMeSubsystemUpdate method is not implemented")
}

func (s *server) NVMeSubsystemList(ctx context.Context, in *pb.NVMeSubsystemListRequest) (*pb.NVMeSubsystemListResponse, error) {
	log.Printf("NVMeSubsystemList: Received from client: %v", in)
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
	return &pb.NVMeSubsystemListResponse{Subsystem: Blobarray}, nil
}

func (s *server) NVMeSubsystemGet(ctx context.Context, in *pb.NVMeSubsystemGetRequest) (*pb.NVMeSubsystem, error) {
	log.Printf("NVMeSubsystemGet: Received from client: %v", in)
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

func (s *server) NVMeControllerCreate(ctx context.Context, in *pb.NVMeControllerCreateRequest) (*pb.NVMeController, error) {
	log.Printf("NVMeControllerCreate: Received from client: %v", in)
	params := MrvlNvmSubsysCreateCtrlrParams{
		Subnqn: in.Controller.Spec.Id.Value,
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
	response := &pb.NVMeController{Spec: &pb.NVMeControllerSpec{Id: &pc.ObjectKey{Value: "TBD"}}}
	err = deepcopier.Copy(in.Controller).To(response)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	return response, nil
}

func (s *server) NVMeControllerDelete(ctx context.Context, in *pb.NVMeControllerDeleteRequest) (*emptypb.Empty, error) {
	log.Printf("NVMeControllerDelete: Received from client: %v", in)
	controller, ok := controllers[in.ControllerId.Value]
	if !ok {
		return nil, fmt.Errorf("error finding controller %s", in.ControllerId.Value)
	}
	params := MrvlNvmSubsysRemoveCtrlrParams{
		Subnqn: in.GetControllerId().GetValue(),
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

func (s *server) NVMeControllerUpdate(ctx context.Context, in *pb.NVMeControllerUpdateRequest) (*pb.NVMeController, error) {
	log.Printf("NVMeControllerUpdate: Received from client: %v", in)
	params := MrvlNvmSubsysCreateCtrlrParams{
		Subnqn: in.Controller.Spec.Id.Value,
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

func (s *server) NVMeControllerList(ctx context.Context, in *pb.NVMeControllerListRequest) (*pb.NVMeControllerListResponse, error) {
	log.Printf("NVMeControllerList: Received from client: %v", in)
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
	return &pb.NVMeControllerListResponse{Controller: Blobarray}, nil
}

func (s *server) NVMeControllerGet(ctx context.Context, in *pb.NVMeControllerGetRequest) (*pb.NVMeController, error) {
	log.Printf("NVMeControllerGet: Received from client: %v", in)
	controller, ok := controllers[in.ControllerId.Value]
	if !ok {
		return nil, fmt.Errorf("error finding controller %s", in.ControllerId.Value)
	}
	params := MrvlNvmGetCtrlrInfoParams{
		Subnqn: in.GetControllerId().GetValue(),
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
	params := MrvlNvmGetCtrlrStatsParams{
		Subnqn: in.GetId().GetValue(),
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

func (s *server) NVMeNamespaceCreate(ctx context.Context, in *pb.NVMeNamespaceCreateRequest) (*pb.NVMeNamespace, error) {
	log.Printf("NVMeNamespaceCreate: Received from client: %v", in)
	params := MrvlNvmSubsysAllocNsParams{
		Subnqn: in.Namespace.Spec.SubsystemId.Value,
		Bdev:   in.Namespace.Spec.VolumeId.Value,
	}

	var result MrvlNvmSubsysAllocNsResult
	err := call("mrvl_nvm_subsys_alloc_ns", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	namespaces[in.Namespace.Spec.Id.Value] = in.Namespace

	response := &pb.NVMeNamespace{}
	err = deepcopier.Copy(in.Namespace).To(response)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	return response, nil
}

func (s *server) NVMeNamespaceDelete(ctx context.Context, in *pb.NVMeNamespaceDeleteRequest) (*emptypb.Empty, error) {
	log.Printf("NVMeNamespaceDelete: Received from client: %v", in)
	namespace, ok := namespaces[in.NamespaceId.Value]
	if !ok {
		err := fmt.Errorf("unable to find key %s", in.NamespaceId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}
	// TODO: replace with MRVL code
	subsys, ok := subsystems[namespace.Spec.SubsystemId.Value]
	if !ok {
		err := fmt.Errorf("unable to find subsystem %s", namespace.Spec.SubsystemId.Value)
		log.Printf("error: %v", err)
		// TODO: temp workaround
		subsys = &pb.NVMeSubsystem{Spec: &pb.NVMeSubsystemSpec{Nqn: namespace.Spec.SubsystemId.Value}}
		// return nil, err
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

func (s *server) NVMeNamespaceUpdate(ctx context.Context, in *pb.NVMeNamespaceUpdateRequest) (*pb.NVMeNamespace, error) {
	log.Printf("NVMeNamespaceUpdate: Received from client: %v", in)
	return nil, status.Errorf(codes.Unimplemented, "NVMeNamespaceUpdate method is not implemented")
}

func (s *server) NVMeNamespaceList(ctx context.Context, in *pb.NVMeNamespaceListRequest) (*pb.NVMeNamespaceListResponse, error) {
	log.Printf("NVMeNamespaceList: Received from client: %v", in)
	params := MrvlNvmSubsysGetNsListParams{
		Subnqn: in.SubsystemId.Value,
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
	return &pb.NVMeNamespaceListResponse{Namespace: Blobarray}, nil
}

func (s *server) NVMeNamespaceGet(ctx context.Context, in *pb.NVMeNamespaceGetRequest) (*pb.NVMeNamespace, error) {
	log.Printf("NVMeNamespaceGet: Received from client: %v", in)
	params := MrvlNvmGetNsInfoParams{
		SubNqn: in.GetNamespaceId().GetValue(),
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
	params := MrvlNvmGetNsStatsParams{
		SubNqn: in.GetNamespaceId().GetValue(),
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
