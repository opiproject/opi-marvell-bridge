// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Dell Inc, or its subsidiaries.
// Copyright (C) 2022 Marvell International Ltd.

package main

type MrvlNvmInitResult struct {
	Status int `json:"status"`
}

type MrvlNvmCreateSubsystemParams struct {
	SubNqn        string `json:"subnqn"`
	ModelNo       string `json:"mn"`
	SerialNo      string `json:"sn"`
	MaxNamespaces int    `json:"max_namespaces"`
	MinCtrlId     int    `json:"min_ctrlr_id"`
	MaxCtrlId     int    `json:"max_ctrlr_id"`
}

type MrvlNvmCreateSubsystemResult struct {
	Status int `json:"status"`
}

type MrvvNvmGetSubsysListResult struct {
	Status     int `json:"status"`
	SubsysList []struct {
		SubNqn string `json:"subnqn"`
	} `json:"subsys_list"`
}

type MrvlNvmSubsysCreateCtrlrParams struct {
	Subnqn string `json:"subnqn"`
}

type MrvlNvmSubsysCreateCtrlrResult struct {
	Status  int `json:"status"`
	CtrlrId int `json:"ctrlr_id"`
}

type MrvlNvmSubsysGetCtrlrListParams struct {
	Subnqn string `json:"subnqn"`
}

type MrvlNvmSubsysGetCtrlrListResult struct {
	Status      int `json:"status"`
	CtrlrIdList []struct {
		CtrlrId int `json:"ctrlr_id"`
	} `json:"ctrlr_id_list"`
}

type MrvlNvmSubsysRemoveCtrlrParams struct {
	Subnqn  string `json:"subnqn"`
	CtrlrId int    `json:"ctrlr_id"`
}

type MrvlNvmSubsysRemoveCtrlrResult struct {
	Status int `json:"status"`
}

type MrvlNvmSubsysAllocNsParams struct {
	Subnqn       string `json:"subnqn"`
	Bdev         string `json:"bdev"`
	Nguid        string `json:"nguid,omitempty"`
	Eui64        string `json:"eui64,omitempty"`
	Uuid         string `json:"uuid,omitempty"`
	NsInstanceId string `json:"ns_instance_id,omitempty"`
	ShareEnable  string `json:"share_enable,omitempty"`
}

type MrvlNvmSubsysAllocNsResult struct {
	Status int `json:"status"`
	NsId   int `json:"ns_instance_id"`
}

type MrvlNvmSubsysUnallocNsParams struct {
	Subnqn string `json:"subnqn"`
	NsId   int    `json:"ns_instance_id"`
}

type MrvlNvmSubsysUnallocNsResult struct {
	Status int `json:"status"`
}

type MrvlNvmCtrlrAttachNsParams struct {
	Subnqn  string `json:"subnqn"`
	CtrlrId int    `json:"ctrlr_id"`
	NsId    int    `json:"ns_instance_id"`
}

type MrvlNvmCtrlrAttachNsResult struct {
	Status int `json:"status"`
}

type MrvlNvmCtrlrDetachNsParams struct {
	Subnqn  string `json:"subnqn"`
	CtrlrId int    `json:"ctrlr_id"`
	NsId    int    `json:"ns_instance_id"`
}

type MrvlNvmCtrlrDetachNsResult struct {
	Status int `json:"status"`
}
