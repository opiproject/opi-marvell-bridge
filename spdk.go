// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2022 Dell Inc, or its subsidiaries.
// Copyright (C) 2022 Marvell International Ltd.

package main

// GetVersionResult represents a Marvell get version result
type GetVersionResult struct {
	Version string `json:"version"`
	Fields  struct {
		Major  int    `json:"major"`
		Minor  int    `json:"minor"`
		Patch  int    `json:"patch"`
		Suffix string `json:"suffix"`
	} `json:"fields"`
}

// MrvlNvmGetSubsysCountParams is empty

// MrvlNvmGetSubMrvvNvmGetSubsysListParams is empty

// MrvlNvmGetSubsysListResult represents a Marvell subsystem list result
type MrvlNvmGetSubsysListResult struct {
	Status     int `json:"status"`
	SubsysList []struct {
		Subnqn string `json:"subnqn"`
	} `json:"subsys_list"`
}

// MrvlNvmCreateSubsystemParams represents the parameters to a Marvell create subsystem request
type MrvlNvmCreateSubsystemParams struct {
	Subnqn        string `json:"subnqn"`
	Mn            string `json:"mn"`
	Sn            string `json:"sn"`
	MaxNamespaces int    `json:"max_namespaces"`
	MinCtrlrID    int    `json:"min_ctrlr_id"`
	MaxCtrlrID    int    `json:"max_ctrlr_id"`
}

// MrvlNvmCreateSubsystemResult represents a Marvell create subsystem result
type MrvlNvmCreateSubsystemResult struct {
	Status int `json:"status"`
}

// MrvlNvmDeleteSubsystemParams represents the parameters to a Marvell delete subsystem request
type MrvlNvmDeleteSubsystemParams struct {
	Subnqn string `json:"subnqn"`
}

// MrvlNvmDeleteSubsystemResult represents a Marvell delete subsystem result
type MrvlNvmDeleteSubsystemResult struct {
	Status int `json:"status"`
}

// MrvlNvmDeInitParams is empty

// MrvlNvmDeInitResult represents a Marvell de-init result
type MrvlNvmDeInitResult struct {
	Status int `json:"status"`
}

// MrvlNvmGetSubsysInfoParams represents the parameters to a Marvell get subsystem info request
type MrvlNvmGetSubsysInfoParams struct {
	Subnqn string `json:"subnqn"`
}

// MrvlNvmGetSubsysInfoResult represents a Marvell get subsystem info result
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

// MrvlNvmSubsysAllocNsParams represents the parameters to a Marvell get subsystem allocate namespace request
type MrvlNvmSubsysAllocNsParams struct {
	Subnqn       string `json:"subnqn"`
	Nguid        string `json:"nguid"`
	Eui64        string `json:"eui64"`
	UUID         string `json:"uuid"`
	NsInstanceID int    `json:"ns_instance_id"`
	ShareEnable  int    `json:"share_enable"`
	Bdev         string `json:"bdev"`
}

// MrvlNvmSubsysAllocNsResult represents a Marvell get subsystem alloc namespace result
type MrvlNvmSubsysAllocNsResult struct {
	Status       int `json:"status"`
	NsInstanceID int `json:"ns_instance_id"`
}

// MrvlNvmSubsysUnallocNsParams represents the parameters to a Marvell get subsystem unallocate namespace request
type MrvlNvmSubsysUnallocNsParams struct {
	Subnqn       string `json:"subnqn"`
	NsInstanceID int    `json:"ns_instance_id"`
}

// MrvlNvmSubsysUnallocNsResult represents a Marvell get subsystem unalloc namespace result
type MrvlNvmSubsysUnallocNsResult struct {
	Status int `json:"status"`
}

// MrvlNvmSubsysGetNsListParams represents the parameters to a Marvell get subsystem namespace list request
type MrvlNvmSubsysGetNsListParams struct {
	Subnqn string `json:"subnqn"`
}

// MrvlNvmSubsysGetNsListResult represents a Marvell get subsystem namespace list result
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

// MrvlNvmSubsysCreateCtrlrParams represents the parameters to a Marvell create subsystem controller request
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

// MrvlNvmSubsysCreateCtrlrResult represents a Marvell create subsystem controller result
type MrvlNvmSubsysCreateCtrlrResult struct {
	Status  int `json:"status"`
	CtrlrID int `json:"ctrlr_id"`
}

// MrvlNvmSubsysUpdateCtrlrParams represents the parameters to a Marvell update subsystem controller request
type MrvlNvmSubsysUpdateCtrlrParams struct {
	Subnqn  string `json:"subnqn"`
	CtrlrID int    `json:"ctrlr_id"`
	MaxNsq  int    `json:"max_nsq"`
	MaxNcq  int    `json:"max_ncq"`
}

// MrvlNvmSubsysUpdateCtrlrResult represents  a Marvell update subsystem controller result
type MrvlNvmSubsysUpdateCtrlrResult struct {
	Status int `json:"status"`
}

// MrvlNvmSubsysRemoveCtrlrParams represents the parameters to a Marvell remove subsystem controller request
type MrvlNvmSubsysRemoveCtrlrParams struct {
	Subnqn  string `json:"subnqn"`
	CntlrID int    `json:"cntlr_id"`
}

// MrvlNvmSubsysRemoveCtrlrResult represents  a Marvell remove subsystem controller result
type MrvlNvmSubsysRemoveCtrlrResult struct {
	Status int `json:"status"`
}

// MrvlNvmSubsysGetCtrlrListParams represents the parameters to a Marvell get subsystem controller list request
type MrvlNvmSubsysGetCtrlrListParams struct {
	Subnqn string `json:"subnqn"`
}

// MrvlNvmSubsysGetCtrlrListResult represents a Marvell get subsystem controller list result
type MrvlNvmSubsysGetCtrlrListResult struct {
	Status      int `json:"status"`
	CtrlrIDList []struct {
		CtrlrID int `json:"ctrlr_id"`
	} `json:"ctrlr_id_list"`
}

// MrvlNvmGetNsStatsParams represents the parameters to a Marvell get namespace status request
type MrvlNvmGetNsStatsParams struct {
	SubNqn       string `json:"subnqn"`
	NsInstanceID int    `json:"ns_instance_id"`
}

// MrvlNvmGetNsStatsResult represents a Marvell get namespace status result
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

// MrvlNvmNsGetCtrlrListParams represents the parameters to a Marvell get namespace controller list request
type MrvlNvmNsGetCtrlrListParams struct {
	SubNqn       string `json:"subnqn"`
	NsInstanceID int    `json:"ns_instance_id"`
}

// MrvlNvmNsGetCtrlrListResult represents a Marvell get namespace controller list result
type MrvlNvmNsGetCtrlrListResult struct {
	Status      int `json:"status"`
	CtrlrIDList []struct {
		CtrlrID int `json:"ctrlr_id"`
	} `json:"ctrlr_id_list"`
}

// MrvlNvmGetNsInfoParams represents the parameters to a Marvell get namespace info request
type MrvlNvmGetNsInfoParams struct {
	SubNqn       string `json:"subnqn"`
	NsInstanceID int    `json:"ns_instance_id"`
}

// MrvlNvmGetNsInfoResult represents the a Marvell get namespace info result
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

// MrvlNvmCtrlrAttachNsParams represents the parameters to a Marvell controller attach namespace request
type MrvlNvmCtrlrAttachNsParams struct {
	Subnqn       string `json:"subnqn"`
	CtrlID       int    `json:"ctrl_id"`
	NsInstanceID int    `json:"ns_instance_id"`
}

// MrvlNvmCtrlrAttachNsResult represents a Marvell controller attach namespace result
type MrvlNvmCtrlrAttachNsResult struct {
	Status int `json:"status"`
}

// MrvlNvmCtrlrDetachNsParams represents the parameters to a Marvell controller detach namespace request
type MrvlNvmCtrlrDetachNsParams struct {
	Subnqn       string `json:"subnqn"`
	CtrlrID      int    `json:"ctrlr_id"`
	NsInstanceID int    `json:"ns_instance_id"`
}

// MrvlNvmCtrlrDetachNsResult represents a Marvell controller detach namespace result
type MrvlNvmCtrlrDetachNsResult struct {
	Status int `json:"status"`
}

// MrvlNvmGetCtrlrInfoParams represents the parameters to a Marvell get controller info request
type MrvlNvmGetCtrlrInfoParams struct {
	Subnqn  string `json:"subnqn"`
	CtrlrID int    `json:"ctrlr_id"`
}

// MrvlNvmGetCtrlrInfoResult represents a Marvell get controller info result
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

// MrvlNvmGetCtrlrStatsParams represents the parameters to a Marvell get controller status request
type MrvlNvmGetCtrlrStatsParams struct {
	Subnqn  string `json:"subnqn"`
	CtrlrID int    `json:"ctrlr_id"`
}

// MrvlNvmGetCtrlrStatsResult represents a Marvell get controller status result
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

// MrvlNvmCtrlrGetNsStatsParams represents the parameters to a Marvell get namespace status request
type MrvlNvmCtrlrGetNsStatsParams struct {
	Subnqn       string `json:"subnqn"`
	CtrlrID      int    `json:"ctrlr_id"`
	NsInstanceID int    `json:"ns_instance_id"`
}

// MrvlNvmCtrlrGetNsStatsResult represents a Marvell get namespace status result
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
