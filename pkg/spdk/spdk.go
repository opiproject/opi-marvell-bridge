// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Dell Inc, or its subsidiaries.
// Copyright (C) 2022 Marvell International Ltd.

package spdk

// MrvlNvmInitParams is empty

type MrvlNvmInitResult struct {
	Status int `json:"status"`
}

// MrvlNvmGetOffloadCapParams is empty

type MrvlNvmGetOffloadCapResult struct {
	Status            int    `json:"status"`
	SdkVersion        string `json:"sdk_version"`
	NvmVersion        string `json:"nvm_version"`
	NumPcieDomains    int    `json:"num_pcie_domains"`
	NumPfsPerDomain   int    `json:"num_pfs_per_domain"`
	NumVfsPerPf       int    `json:"num_vfs_per_pf"`
	TotalIoqPerPf     int    `json:"total_ioq_per_pf"`
	MaxIoqPerPf       int    `json:"max_ioq_per_pf"`
	MaxIoqPerVf       int    `json:"max_ioq_per_vf"`
	MaxSubsystems     int    `json:"max_subsystems"`
	MaxNsPerSubsys    int    `json:"max_ns_per_subsys"`
	MaxCtrlrPerSubsys int    `json:"max_ctrlr_per_subsys"`
}

// MrvlNvmGetSubsysCountParams is empty

type MrvlNvmGetSubsysCountResult struct {
	Status int `json:"status"`
	Count  int `json:"count"`
}

// MrvlNvmGetSubMrvvNvmGetSubsysListParams is empty

type MrvvNvmGetSubsysListResult struct {
	Status     int `json:"status"`
	SubsysList []struct {
		Subnqn string `json:"subnqn"`
	} `json:"subsys_list"`
}

type MrvlNvmCreateSubsystemParams struct {
	Subnqn        string `json:"subnqn"`
	Mn            string `json:"mn"`
	Sn            string `json:"sn"`
	MaxNamespaces int    `json:"max_namespaces"`
	MinCtrlrID    int    `json:"min_ctrlr_id"`
	MaxCtrlrID    int    `json:"max_ctrlr_id"`
}

type MrvlNvmCreateSubsystemResult struct {
	Status int `json:"status"`
}

type MrvlNvmDeleteSubsystemParams struct {
	Subnqn string `json:"subnqn"`
}

type MrvlNvmDeleteSubsystemResult struct {
	Status int `json:"status"`
}

// MrvlNvmDeInitParams is empty

type MrvlNvmDeInitResult struct {
	Status int `json:"status"`
}

type MrvlNvmGetSubsysInfoParams struct {
	Subnqn string `json:"subnqn"`
}

type MrvlNvmGetSubsysInfoResult struct {
	Status     int `json:"status"`
	SubsysList []struct {
		Subnqn         string `json:"subnqn"`
		Mn             string `json:"mn"`
		Sn             string `json:"sn"`
		MaxNamespaces  int    `json:"max_namespaces"`
		MinCtrlrID     int    `json:"min_ctrlr_id"`
		MaxCtrlrID     int    `json:"max_ctrlr_id"`
		NumNs          int    `json:"num_ns"`
		NumTotalCtrlr  int    `json:"num_total_ctrlr"`
		NumActiveCtrlr int    `json:"num_active_ctrlr"`
		NsList         []struct {
			NsInstanceID int    `json:"ns_instance_id"`
			Bdev         string `json:"bdev"`
			CtrlrIDList  []struct {
				CtrlrID int `json:"ctrlr_id"`
			} `json:"ctrlr_id_list"`
		} `json:"ns_list"`
	} `json:"subsys_list"`
}

type MrvlNvmSubsysAllocNsParams struct {
	Subnqn       string `json:"subnqn"`
	Nguid        string `json:"nguid"`
	Eui64        string `json:"eui64"`
	UUID         string `json:"uuid"`
	NsInstanceID int    `json:"ns_instance_id"`
	ShareEnable  int    `json:"share_enable"`
	Bdev         string `json:"bdev"`
}

type MrvlNvmSubsysAllocNsResult struct {
	Status       int `json:"status"`
	NsInstanceID int `json:"ns_instance_id"`
}

type MrvlNvmSubsysUnallocNsParams struct {
	Subnqn       string `json:"subnqn"`
	NsInstanceID int    `json:"ns_instance_id"`
}

type MrvlNvmSubsysUnallocNsResult struct {
	Status int `json:"status"`
}

type MrvlNvmSubsysGetNsListParams struct {
	Subnqn string `json:"subnqn"`
}

type MrvlNvmSubsysGetNsListResult struct {
	Status int `json:"status"`
	NsList []struct {
		NsInstanceID int    `json:"ns_instance_id"`
		Bdev         string `json:"bdev"`
		CtrlrIDList  []struct {
			CtrlrID int `json:"ctrlr_id"`
		} `json:"ctrlr_id_list"`
	} `json:"ns_list"`
}

type MrvlNvmSubsysCreateCtrlrParams struct {
	Subnqn       string `json:"subnqn"`
	PcieDomainID int    `json:"pcie_domain_id"`
	PfID         int    `json:"pf_id"`
	VfID         int    `json:"vf_id"`
	CtrlID       int    `json:"ctrl_id"`
	Sqes         int    `json:"sqes"`
	Cqes         int    `json:"cqes"`
	MaxNsq       int    `json:"max_nsq"`
	MaxNcq       int    `json:"max_ncq"`
}

type MrvlNvmSubsysCreateCtrlrResult struct {
	Status  int `json:"status"`
	CtrlrID int `json:"ctrlr_id"`
}

type MrvlNvmSubsysUpdateCtrlrParams struct {
	Subnqn  string `json:"subnqn"`
	CtrlrID int    `json:"ctrlr_id"`
	MaxNsq  int    `json:"max_nsq"`
	MaxNcq  int    `json:"max_ncq"`
}

type MrvlNvmSubsysUpdateCtrlrResult struct {
	Status int `json:"status"`
}

type MrvlNvmSubsysRemoveCtrlrParams struct {
	Subnqn  string `json:"subnqn"`
	CntlrID int    `json:"cntlr_id"`
}

type MrvlNvmSubsysRemoveCtrlrResult struct {
	Status int `json:"status"`
}

type MrvlNvmSubsysGetCtrlrListParams struct {
	Subnqn string `json:"subnqn"`
}

type MrvlNvmSubsysGetCtrlrListResult struct {
	Status      int `json:"status"`
	CtrlrIDList []struct {
		CtrlrID int `json:"ctrlr_id"`
	} `json:"ctrlr_id_list"`
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

type MrvlNvmnSGetCtrlrListParams struct {
	SubNqn       string `json:"subnqn"`
	NsInstanceID int    `json:"ns_instance_id"`
}

type MrvlNvmnSGetCtrlrListResult struct {
	Status      int `json:"status"`
	CtrlrIDList []struct {
		CtrlrID int `json:"ctrlr_id"`
	} `json:"ctrlr_id_list"`
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

type MrvlNvmCtrlrAttachNsParams struct {
	Subnqn       string `json:"subnqn"`
	CtrlID       int    `json:"ctrl_id"`
	NsInstanceID int    `json:"ns_instance_id"`
}

type MrvlNvmCtrlrAttachNsResult struct {
	Status int `json:"status"`
}

type MrvlNvmCtrlrDetachNsParams struct {
	Subnqn       string `json:"subnqn"`
	CtrlrID      int    `json:"ctrlr_id"`
	NsInstanceID int    `json:"ns_instance_id"`
}

type MrvlNvmCtrlrDetachNsResult struct {
	Status int `json:"status"`
}

type MrvlNvmGetCtrlrInfoParams struct {
	Subnqn  string `json:"subnqn"`
	CtrlrID int    `json:"ctrlr_id"`
}

type MrvlNvmGetCtrlrInfoResult struct {
	Status        int    `json:"status"`
	PcieDomainID  int    `json:"pcie_domain_id"`
	PfID          int    `json:"pf_id"`
	VfID          int    `json:"vf_id"`
	CtrlrID       int    `json:"ctrlr_id"`
	MaxNsq        int    `json:"max_nsq"`
	MaxNcq        int    `json:"max_ncq"`
	Mqes          int    `json:"mqes"`
	IeeeOui       string `json:"ieee_oui"`
	Cmic          int    `json:"cmic"`
	Nn            int    `json:"nn"`
	ActiveNsCount int    `json:"active_ns_count"`
	ActiveNsq     int    `json:"active_nsq"`
	ActiveNcq     int    `json:"active_ncq"`
	Mdts          int    `json:"mdts"`
	Sqes          int    `json:"sqes"`
	Cqes          int    `json:"cqes"`
}

type MrvlNvmGetCtrlrStatsParams struct {
	Subnqn  string `json:"subnqn"`
	CtrlrID int    `json:"ctrlr_id"`
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
	StatsTimeWindowInUs   int `json:"Stats_time_window_in_us"`
}

type MrvlNvmCtrlrGetNsStatsParams struct {
	Subnqn       string `json:"subnqn"`
	CtrlrID      int    `json:"ctrlr_id"`
	NsInstanceID int    `json:"ns_instance_id"`
}

type MrvlNvmCtrlrGetNsStatsResult struct {
	Status                int `json:"status"`
	NumReadCmds           int `json:"num_read_cmds"`
	NumReadBytes          int `json:"num_read_bytes"`
	NumWriteCmds          int `json:"num_write_cmds"`
	NumWriteBytes         int `json:"num_write_bytes"`
	NumErrors             int `json:"num_errors"`
	TotalReadLatencyInUs  int `json:"total_read_latency_in_us"`
	TotalWriteLatencyInUs int `json:"total_write_latency_in_us"`
	StatsTimeWindowInUs   int `json:"stats_time_window_in_us"`
}
