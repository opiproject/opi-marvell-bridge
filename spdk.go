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

type MrvlNvmDeleteSubsystemParams struct {
	SubNqn        string `json:"subnqn"`
}

type MrvlNvmDeleteSubsystemResult struct {
	Status int `json:"status"`
}

type MrvvNvmGetSubsysListResult struct {
	Status     int `json:"status"`
	SubsysList []struct {
		SubNqn string `json:"subnqn"`
	} `json:"subsys_list"`
}

type MrvlNvmGetSubsysInfoParams struct {
	SubNqn        string `json:"subnqn"`
}

type MrvlNvmGetSubsysInfoResult struct {
	Status int `json:"status"`
	SubsysList []struct {
		SubNqn string `json:"subnqn"`
		ModelNo       string `json:"mn"`
		SerialNo      string `json:"sn"`
		MaxNamespaces int    `json:"max_namespaces"`
		MinCtrlId     int    `json:"min_ctrlr_id"`
		MaxCtrlId     int    `json:"max_ctrlr_id"`
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

type MrvlNvmGetCtrlrInfoParams struct {
	SubNqn  string `json:"subnqn"`
	CtrlrId int `json:"ctrlr_id"`
}

type MrvlNvmGetCtrlrInfoResult struct {
	Status        int `json:"status"`
	PcieDomainId  int `json:"pcie_domain_id"`
	PfId          int `json:"pf_id"`
	VfId          int `json:"vf_id"`
	CtrlrId       int `json:"ctrlr_id"`
	MaxNsq        int `json:"max_nsq"`
	MaxNcq        int `json:"max_ncq"`
	Mqes          int `json:"mqes"`
	IeeeOui       string `json:"005043"`
	Cmic          int `json:"cmic"`
	Nn            int `json:"nn"`
	ActiveNsCount int `json:"active_ns_count"`
	ActiveNsq     int `json:"active_nsq"`
	ActiveNcq     int `json:"active_ncq"`
	Mdts          int `json:"mdts"`
	Sqes          int `json:"sqes"`
	Cqes          int `json:"cqes"`
}

type MrvlNvmSubsysRemoveCtrlrParams struct {
	Subnqn  string `json:"subnqn"`
	CtrlrId int    `json:"ctrlr_id"`
}

type MrvlNvmSubsysRemoveCtrlrResult struct {
	Status int `json:"status"`
}

type MrvlNvmGetCtrlrStatsParams struct {
	SubNqn  string `json:"subnqn"`
	CtrlrId int `json:"ctrlr_id"`
}

type MrvlNvmGetCtrlrStatsResult struct {
	Status                int `json:"status"`
	NumAdminCmds          int `json:"num_admin_cmds"`
	NumAdminCmdErrors     int `json:"num_admin_cmd_errors"`
	NumAsyncEvents        int `json:"num_async_events"`
	NumReadCmds           int `json:"num_read_cmds"`
	NumReadBytes          int `json:"num_read_bytes"`
	NumWriteCmds          int `json:"num_write_cmds"`
	NumWriteBytes         int `json:"num_write_bytes"`
	NumErrors             int `json:"num_errors"`
	TotalReadLatencyInUs  int `json:"total_read_latency_in_us"`
	TotalWriteLatencyInUs int `json:"total_write_latency_in_us"`
	StatsTimeWindowInUs   int `json:"stats_time_window_in_us"`
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

type MrvlNvmSubsysGetNsListParams struct {
	Subnqn string `json:"subnqn"`
}

type MrvlNvmSubsysGetNsListResult struct {
	Status      int `json:"status"`
	NsList []struct {
		NsInstanceID int    `json:"ns_instance_id"`
		Bdev         string `json:"bdev"`
		CtrlrIDList  []struct {
			CtrlrID int `json:"ctrlr_id"`
		} `json:"ctrlr_id_list"`
	} `json:"ns_list"`
}

type MrvlNvmGetNsStatsParams struct {
	SubNqn       string `json:"subnqn"`
	NsInstanceID int    `json:"ns_instance_id"`
}

type MrvlNvmGetNsStatsResult struct {
	Status                int `json:"status"`
	NumReadCmds           int `json:"num_read_cmds"`
	NumReadBytes          int `json:"num_read_bytes"`
	NumWriteCmds          int `json:"num_write_cmds"`
	NumWriteBytes         int `json:"num_write_bytes"`
	NumErrors             int `json:"num_errors"`
	TotalReadLatencyInUs  int `json:"total_read_latency_in_us"`
	TotalWriteLatencyInUs int `json:"total_write_latency_in_us"`
	StatsTimeWindowInUs   int `json:"Stats_time_window_in_us"`
}

type MrvlNvmGetNsInfoParams struct {
	SubNqn       string `json:"subnqn"`
	NsInstanceID int    `json:"ns_instance_id"`
}

type MrvlNvmGetNsInfoResult struct {
	Status      int    `json:"status"`
	Nguid       string `json:"nguid"`
	Eui64       string `json:"eui64"`
	UUID        string `json:"uuid"`
	Nmic        int    `json:"nmic"`
	Bdev        string `json:"bdev"`
	NumCtrlrs   int    `json:"num_ctrlrs"`
	CtrlrIDList []struct {
		CtrlrID int `json:"ctrlr_id"`
	} `json:"ctrlr_id_list"`
}
