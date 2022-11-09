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

type server string

var MrvlFrontendNvmeService server

// ////////////////////////////////////////////////////////
var subsystems = map[string]*pb.NVMeSubsystem{}

func (s *server) NVMeSubsystemCreate(ctx context.Context, in *pb.NVMeSubsystemCreateRequest) (*pb.NVMeSubsystem, error) {
	log.Printf("NVMeSubsystemCreate: Received from client: %v", in)
	params := MrvlNvmCreateSubsystemParams{
		SubNqn:       in.GetSubsystem().GetNqn(),
		ModelNo: 	  "OpiModel0",
		SerialNo:     "OpiSerial0",
	}
	var result MrvlNvmCreateSubsystemResult
	err := call("mrvl_nvm_create_subsystem", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	subsystems[in.Subsystem.Id.Value] = in.Subsystem
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
		SubNqn: subsys.Nqn,
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
	delete(subsystems, subsys.Id.Value)
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
		r := &result[i]
		Blobarray[i] = &pb.NVMeSubsystem{Nqn: r.SubNqn}
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
		if r.SubNqn == subsys.Nqn {
			return &pb.NVMeSubsystem{Nqn: r.SubNqn}, nil
		}
	}
	msg := fmt.Sprintf("Could not find NQN: %s", subsys.Nqn)
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
		SubNqn: subsys.Nqn,
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
	log.Printf("Received from client: %v", in.Controller)
	params := MrvlNvmSubsysCreateCtrlrParams{
		Nqn:          in.GetSubsystem().GetNqn(),
		SerialNumber: "SPDK0",
		AllowAnyHost: true,
	}
	var result MrvlNvmSubsysCreateCtrlrResult
	err := call("mrvl_nvm_subsys_create_ctrlr", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	controllers[in.Controller.Id.Value] = in.Controller
	response := &pb.NVMeController{}
	err := deepcopier.Copy(in.Controller).To(response)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	return response, nil
}

func (s *server) NVMeControllerDelete(ctx context.Context, in *pb.NVMeControllerDeleteRequest) (*emptypb.Empty, error) {
	log.Printf("Received from client: %v", in.ControllerId)
	controller, ok := controllers[in.ControllerId.Value]
	if !ok {
		return nil, fmt.Errorf("error finding controller %s", in.ControllerId.Value)
	}
	params := NvmfDeleteSubsystemParams{
		Nqn: subsys.Nqn,
	}
	var result NvmfDeleteSubsystemResult
	err := call("mrvl_nvm_subsys_remove_ctrlr", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	if !result {
		log.Printf("Could not delete: %v", in)
	}
	delete(controllers, controller.Id.Value)
	return &emptypb.Empty{}, nil
}

func (s *server) NVMeControllerUpdate(ctx context.Context, in *pb.NVMeControllerUpdateRequest) (*pb.NVMeController, error) {
	log.Printf("Received from client: %v", in.Controller)
	params := MrvlNvmSubsysCreateCtrlrParams{
		Nqn:          in.GetSubsystem().GetNqn(),
		SerialNumber: "SPDK0",
		AllowAnyHost: true,
	}
	var result MrvlNvmSubsysCreateCtrlrResult
	err := call("mrvl_nvm_subsys_update_ctrlr", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	controllers[in.Controller.Id.Value] = in.Controller
	// TODO: replace with MRVL code
	response := &pb.NVMeController{}
	err := deepcopier.Copy(in.Controller).To(response)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	return response, nil
}

func (s *server) NVMeControllerList(ctx context.Context, in *pb.NVMeControllerListRequest) (*pb.NVMeControllerListResponse, error) {
	log.Printf("Received from client: %v", in.SubsystemId)
	// TODO: replace with MRVL code
	var result []MrvvNvmGetSubsysListResult
	err := call("mrvl_nvm_subsys_get_ctrlr_list", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	Blobarray := make([]*pb.NVMeController, len(result))
	for i := range result {
		r := &result[i]
		Blobarray[i] = &pb.NVMeController{Nqn: r.Nqn}
	}
	return &pb.NVMeControllerListResponse{Controller: Blobarray}, nil
}

func (s *server) NVMeControllerGet(ctx context.Context, in *pb.NVMeControllerGetRequest) (*pb.NVMeController, error) {
	log.Printf("Received from client: %v", in.ControllerId)
	// TODO: replace with MRVL code
	controller, ok := controllers[in.ControllerId.Value]
	if !ok {
		return nil, fmt.Errorf("error finding controller %s", in.ControllerId.Value)
	}
	return &pb.NVMeController{Id: in.ControllerId, NvmeControllerId: controller.NvmeControllerId}, nil
}

func (s *server) NVMeControllerStats(ctx context.Context, in *pb.NVMeControllerStatsRequest) (*pb.NVMeControllerStatsResponse, error) {
	log.Printf("Received from client: %v", in.Id)
	params := NvmfDeleteSubsystemParams{
		Nqn: subsys.Nqn,
	}
	var result NvmfDeleteSubsystemResult
	err := call("mrvl_nvm_ns_get_stats", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	return &pb.NVMeControllerStatsResponse{}, nil
}

// ////////////////////////////////////////////////////////
var namespaces = map[string]*pb.NVMeNamespace{}

func (s *server) NVMeNamespaceCreate(ctx context.Context, in *pb.NVMeNamespaceCreateRequest) (*pb.NVMeNamespace, error) {
	log.Printf("NVMeNamespaceCreate: Received from client: %v", in)
	params := MrvlNvmSubsysAllocNsParams{
		Nqn: subsys.Nqn,
	}
	params.Namespace.BdevName = in.Namespace.VolumeId.Value

	var result MrvlNvmSubsysAllocNsResult
	err := call("mrvl_nvm_subsys_alloc_ns", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	namespaces[in.Namespace.Id.Value] = in.Namespace

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
	subsys, ok := subsystems[namespace.SubsystemId.Value]
	if !ok {
		err := fmt.Errorf("unable to find subsystem %s", namespace.SubsystemId.Value)
		log.Printf("error: %v", err)
		// TODO: temp workaround
		subsys = &pb.NVMeSubsystem{Nqn: namespace.SubsystemId.Value}
		// return nil, err
	}

	params := MrvlNvmSubsysUnallocNsParams{
		Nqn:  subsys.Nqn,
		Nsid: int(namespace.HostNsid),
	}
	var result MrvlNvmSubsysUnallocNsResult
	err := call(" mrvl_nvm_subsys_unalloc_ns", &params, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)
	delete(namespaces, namespace.Id.Value)
	return &emptypb.Empty{}, nil
}

func (s *server) NVMeNamespaceUpdate(ctx context.Context, in *pb.NVMeNamespaceUpdateRequest) (*pb.NVMeNamespace, error) {
	log.Printf("Received from client: %v", in.Namespace)
	namespaces[in.Namespace.Id.Value] = in.Namespace
	response := &pb.NVMeNamespace{}
	// TODO: replace with MRVL code
	err := deepcopier.Copy(in.Namespace).To(response)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	return response, nil
}

func (s *server) NVMeNamespaceList(ctx context.Context, in *pb.NVMeNamespaceListRequest) (*pb.NVMeNamespaceListResponse, error) {
	log.Printf("NVMeNamespaceList: Received from client: %v", in)
	var result []NvmfGetSubsystemsResult
	err := call("mrvl_nvm_subsys_get_ns_list", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	log.Printf("Received from SPDK: %v", result)

	Blobarray := []*pb.NVMeNamespace{}
	for i := range result {
		rr := &result[i]
		if rr.Nqn == nqn || nqn == "" {
			for j := range rr.Namespaces {
				r := &rr.Namespaces[j]
				Blobarray = append(Blobarray, &pb.NVMeNamespace{HostNsid: int32(r.Nsid)})
			}
		}
	}
	if len(Blobarray) > 0 {
		return &pb.NVMeNamespaceListResponse{Namespace: Blobarray}, nil
	}

	msg := fmt.Sprintf("Could not find any namespaces for NQN: %s", nqn)
	log.Print(msg)
	return nil, status.Errorf(codes.InvalidArgument, msg)
}

func (s *server) NVMeNamespaceGet(ctx context.Context, in *pb.NVMeNamespaceGetRequest) (*pb.NVMeNamespace, error) {
	log.Printf("NVMeNamespaceGet: Received from client: %v", in)
	// TODO: replace with MRVL code
	namespace, ok := namespaces[in.NamespaceId.Value]
	if !ok {
		err := fmt.Errorf("unable to find key %s", in.NamespaceId.Value)
		log.Printf("error: %v", err)
		return nil, err
	}
	// TODO: do we even query SPDK to confirm if namespace is present?
	// return namespace, nil

	// fetch subsystems -> namespaces from server, match the nsid to find the corresponding namespace
	subsys, ok := subsystems[namespace.SubsystemId.Value]
	if !ok {
		err := fmt.Errorf("unable to find subsystem %s", namespace.SubsystemId.Value)
		log.Printf("error: %v", err)
		// TODO: temp workaround
		subsys = &pb.NVMeSubsystem{Nqn: namespace.SubsystemId.Value}
		// return nil, err
	}

	var result []NvmfGetSubsystemsResult
	err := call("nvmf_get_subsystems", nil, &result)
	if err != nil {
		log.Printf("error: %v", err)
		return nil, err
	}
	// TODO: replace with MRVL code
	log.Printf("Received from SPDK: %v", result)
	for i := range result {
		rr := &result[i]
		if rr.Nqn == subsys.Nqn {
			for j := range rr.Namespaces {
				r := &rr.Namespaces[j]
				if int32(r.Nsid) == namespace.HostNsid {
					return &pb.NVMeNamespace{Id: namespace.Id, HostNsid: namespace.HostNsid}, nil
				}
			}
			msg := fmt.Sprintf("Could not find NSID: %d", namespace.HostNsid)
			log.Print(msg)
			return nil, status.Errorf(codes.InvalidArgument, msg)
		}
	}
	msg := fmt.Sprintf("Could not find NQN: %s", subsys.Nqn)
	log.Print(msg)
	return nil, status.Errorf(codes.InvalidArgument, msg)
}

func (s *server) NVMeNamespaceStats(ctx context.Context, in *pb.NVMeNamespaceStatsRequest) (*pb.NVMeNamespaceStatsResponse, error) {
	log.Printf("Received from client: %v", in.NamespaceId)
	// TODO: replace with MRVL code : mrvl_nvm_ctrlr_get_ns_stats
	return &pb.NVMeNamespaceStatsResponse{}, nil
}

