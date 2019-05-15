package ofp13

import (
	"net"
)

type OFMessage interface {
	Serialize() []byte
	Parse(packet []byte)
	Size() int
}

/**
 * definition for OFProtocol
 */

const OFP_MAX_TABLE_NAME_LEN = 32
const OFP_MAX_PORT_NAME_LEN = 16
const OFP_ETH_ALEN = 6

const (
	OFPP_MAX        = 0xffffff00
	OFPP_IN_PORT    = 0xfffffff8
	OFPP_TABLE      = 0xfffffff9
	OFPP_NORMAL     = 0xfffffffa
	OFPP_FLOOD      = 0xfffffffb
	OFPP_ALL        = 0xfffffffc
	OFPP_CONTROLLER = 0xfffffffd
	OFPP_LOCAL      = 0xfffffffe
	OFPP_ANY        = 0xffffffff
)

const (
	OFPT_HELLO = iota
	OFPT_ERROR
	OFPT_ECHO_REQUEST
	OFPT_ECHO_REPLY
	OFPT_EXPERIMENTER
	OFPT_FEATURES_REQUEST
	OFPT_FEATURES_REPLY
	OFPT_GET_CONFIG_REQUEST
	OFPT_GET_CONFIG_REPLY
	OFPT_SET_CONFIG
	OFPT_PACKET_IN
	OFPT_FLOW_REMOVED
	OFPT_PORT_STATUS
	OFPT_PACKET_OUT
	OFPT_FLOW_MOD
	OFPT_GROUP_MOD
	OFPT_PORT_MOD
	OFPT_TABLE_MOD
	OFPT_MULTIPART_REQUEST
	OFPT_MULTIPART_REPLY
	OFPT_BARRIER_REQUEST
	OFPT_BARRIER_REPLY
	OFPT_QUEUE_GET_CONFIG_REQUEST
	OFPT_QUEUE_GET_CONFIG_REPLY
	OFPT_ROLE_REQUEST
	OFPT_ROLE_REPLY
	OFPT_GET_ASYNC_REQUEST
	OFPT_GET_ASYNC_REPLY
	OFPT_SET_ASYNC
	OFPT_METER_MOD
)

const (
	OFPHET_VERSIONBITMAP = 1
)

const (
	OFPC_FLAG_NORMAL = 0
	OFPC_FLAG_DROP   = 1 << 0
	OFPC_FLAG_REASM  = 1 << 1
	OFPC_FLAG_MASK   = 3
)

const (
	OFPTC_DEPRECATED_MASK = 3
)

const (
	OFPTT_MAX = 0xfe
	OFPTT_ALL = 0xff
)

const (
	OFPC_FLOW_STATS   = 1 << 0
	OFPC_TABLE_STATS  = 1 << 1
	OFPC_PORT_STATS   = 1 << 2
	OFPC_GROUP_STATS  = 1 << 3
	OFPC_IP_REASM     = 1 << 5
	OFPC_QUEUE_STATS  = 1 << 6
	OFPC_PORT_BLOCKED = 1 << 8
)

const (
	OFPPC_PORT_DOWN    = 1 << 0
	OFPPC_NO_RECV      = 1 << 2
	OFPPC_NO_FWD       = 1 << 5
	OFPPC_NO_PACKET_IN = 1 << 6
)

const (
	OFPPS_LINK_DOWN = 1 << 0
	OFPPS_BLOCKED   = 1 << 1
	OFPPS_LIVE      = 1 << 2
)

const (
	OFPPF_10MB_HD  = 1 << 0
	OFPPF_10MB_FD  = 1 << 1
	OFPPF_100MB_HD = 1 << 2
	OFPPF_100MB_FD = 1 << 3
	OFPPF_1GB_HD   = 1 << 4
	OFPPF_1GB_FD   = 1 << 5
	OFPPF_10GB_FD  = 1 << 6
	OFPPF_40GB_FD  = 1 << 7
	OFPPF_100GB_FD = 1 << 8
	OFPPF_1TB_FD   = 1 << 9
	OFPPF_OTHER    = 1 << 10

	OFPPF_COPPER     = 1 << 11
	OFPPF_FIBER      = 1 << 12
	OFPPF_AUTONEG    = 1 << 13
	OFPPF_PAUSE      = 1 << 14
	OFPPF_PAUSE_ASYM = 1 << 15
)

// ofp_port_reason
const (
	OFPPR_ADD = iota
	OFPPR_DELETE
	OFPPR_MODIFY
)

// ofp_match_type
const (
	OFPMT_STANDARD = iota
	OFPMT_OXM
)

// ofp_oxm_class
const (
	OFPXMC_NXM_0          = 0x0000
	OFPXMC_NXM_1          = 0x0001
	OFPXMC_OPENFLOW_BASIC = 0x8000
	OFPXMC_EXPERIMENTER   = 0xffff
)

// oxm_ofb_match_fields
const (
	OFPXMT_OFB_IN_PORT = iota
	OFPXMT_OFB_IN_PHY_PORT
	OFPXMT_OFB_METADATA
	OFPXMT_OFB_ETH_DST
	OFPXMT_OFB_ETH_SRC
	OFPXMT_OFB_ETH_TYPE
	OFPXMT_OFB_VLAN_VID
	OFPXMT_OFB_VLAN_PCP
	OFPXMT_OFB_IP_DSCP
	OFPXMT_OFB_IP_ECN
	OFPXMT_OFB_IP_PROTO
	OFPXMT_OFB_IPV4_SRC
	OFPXMT_OFB_IPV4_DST
	OFPXMT_OFB_TCP_SRC
	OFPXMT_OFB_TCP_DST
	OFPXMT_OFB_UDP_SRC
	OFPXMT_OFB_UDP_DST
	OFPXMT_OFB_SCTP_SRC
	OFPXMT_OFB_SCTP_DST
	OFPXMT_OFB_ICMPV4_TYPE
	OFPXMT_OFB_ICMPV4_CODE
	OFPXMT_OFB_ARP_OP
	OFPXMT_OFB_ARP_SPA
	OFPXMT_OFB_ARP_TPA
	OFPXMT_OFB_ARP_SHA
	OFPXMT_OFB_ARP_THA
	OFPXMT_OFB_IPV6_SRC
	OFPXMT_OFB_IPV6_DST
	OFPXMT_OFB_IPV6_FLABEL
	OFPXMT_OFB_ICMPV6_TYPE
	OFPXMT_OFB_ICMPV6_CODE
	OFPXMT_OFB_IPV6_ND_TARGET
	OFPXMT_OFB_IPV6_ND_SLL
	OFPXMT_OFB_IPV6_ND_TLL
	OFPXMT_OFB_MPLS_LABEL
	OFPXMT_OFB_MPLS_TC
	OFPXMT_OFB_MPLS_BOS
	OFPXMT_OFB_PBB_ISID
	OFPXMT_OFB_TUNNEL_ID
	OFPXMT_OFB_IPV6_EXTHDR
)

func oxmHeader__(class uint32, field uint32, hasMask uint32, length uint32) uint32 {
	return (((class << 16) | (field << 9) | (hasMask << 8)) | (length))
}

func oxmHeader(class uint32, field uint32, length uint32) uint32 {
	return oxmHeader__(class, field, 0, length)
}

func oxmHeaderW(class uint32, field uint32, length uint32) uint32 {
	return oxmHeader__(class, field, 1, length*2)
}

func oxmClass(header uint32) uint32 {
	return header >> 16
}

func oxmField(header uint32) uint32 {
	return ((header >> 9) & 0x7f)
}

func oxmType(header uint32) uint32 {
	return ((header >> 9) & 0x7fffff)
}

func oxmHasMask(header uint32) uint32 {
	return ((header >> 8) & 1)
}

func oxmLength(header uint32) uint32 {
	return (header & 0xff)
}

func oxmMakeWildHeader(header uint32) uint32 {
	return oxmHeaderW(oxmClass(header), oxmField(header), oxmLength(header))
}

const (
	OFPVID_PRESENT = 0x1000
	OFPVID_NONE    = 0x0000
)

const (
	OFP_VLAN_NONE = OFPVID_NONE
)

var OFPXMT_OFB_ALL uint64 = ((1 << 40) - 1)
var OXM_OF_IN_PORT = oxmHeader(0x8000, OFPXMT_OFB_IN_PORT, 4)
var OXM_OF_IN_PHY_PORT = oxmHeader(0x8000, OFPXMT_OFB_IN_PHY_PORT, 4)
var OXM_OF_METADATA = oxmHeader(0x8000, OFPXMT_OFB_METADATA, 8)
var OXM_OF_METADATA_W = oxmHeaderW(0x8000, OFPXMT_OFB_METADATA, 8)
var OXM_OF_ETH_DST = oxmHeader(0x8000, OFPXMT_OFB_ETH_DST, 6)
var OXM_OF_ETH_DST_W = oxmHeaderW(0x8000, OFPXMT_OFB_ETH_DST, 6)
var OXM_OF_ETH_SRC = oxmHeader(0x8000, OFPXMT_OFB_ETH_SRC, 6)
var OXM_OF_ETH_SRC_W = oxmHeaderW(0x8000, OFPXMT_OFB_ETH_SRC, 6)
var OXM_OF_ETH_TYPE = oxmHeader(0x8000, OFPXMT_OFB_ETH_TYPE, 2)
var OXM_OF_VLAN_VID = oxmHeader(0x8000, OFPXMT_OFB_VLAN_VID, 2)
var OXM_OF_VLAN_VID_W = oxmHeaderW(0x8000, OFPXMT_OFB_VLAN_VID, 2)
var OXM_OF_VLAN_PCP = oxmHeader(0x8000, OFPXMT_OFB_VLAN_PCP, 1)
var OXM_OF_IP_DSCP = oxmHeader(0x8000, OFPXMT_OFB_IP_DSCP, 1)
var OXM_OF_IP_ECN = oxmHeader(0x8000, OFPXMT_OFB_IP_ECN, 1)
var OXM_OF_IP_PROTO = oxmHeader(0x8000, OFPXMT_OFB_IP_PROTO, 1)
var OXM_OF_IPV4_SRC = oxmHeader(0x8000, OFPXMT_OFB_IPV4_SRC, 4)
var OXM_OF_IPV4_SRC_W = oxmHeaderW(0x8000, OFPXMT_OFB_IPV4_SRC, 4)
var OXM_OF_IPV4_DST = oxmHeader(0x8000, OFPXMT_OFB_IPV4_DST, 4)
var OXM_OF_IPV4_DST_W = oxmHeaderW(0x8000, OFPXMT_OFB_IPV4_DST, 4)
var OXM_OF_TCP_SRC = oxmHeader(0x8000, OFPXMT_OFB_TCP_SRC, 2)
var OXM_OF_TCP_DST = oxmHeader(0x8000, OFPXMT_OFB_TCP_DST, 2)
var OXM_OF_UDP_SRC = oxmHeader(0x8000, OFPXMT_OFB_UDP_SRC, 2)
var OXM_OF_UDP_DST = oxmHeader(0x8000, OFPXMT_OFB_UDP_DST, 2)
var OXM_OF_SCTP_SRC = oxmHeader(0x8000, OFPXMT_OFB_SCTP_SRC, 2)
var OXM_OF_SCTP_DST = oxmHeader(0x8000, OFPXMT_OFB_SCTP_DST, 2)
var OXM_OF_ICMPV4_TYPE = oxmHeader(0x8000, OFPXMT_OFB_ICMPV4_TYPE, 1)
var OXM_OF_ICMPV4_CODE = oxmHeader(0x8000, OFPXMT_OFB_ICMPV4_CODE, 1)
var OXM_OF_ARP_OP = oxmHeader(0x8000, OFPXMT_OFB_ARP_OP, 2)
var OXM_OF_ARP_SPA = oxmHeader(0x8000, OFPXMT_OFB_ARP_SPA, 4)
var OXM_OF_ARP_SPA_W = oxmHeaderW(0x8000, OFPXMT_OFB_ARP_SPA, 4)
var OXM_OF_ARP_TPA = oxmHeader(0x8000, OFPXMT_OFB_ARP_TPA, 4)
var OXM_OF_ARP_TPA_W = oxmHeaderW(0x8000, OFPXMT_OFB_ARP_TPA, 4)
var OXM_OF_ARP_SHA = oxmHeader(0x8000, OFPXMT_OFB_ARP_SHA, 6)
var OXM_OF_ARP_THA = oxmHeader(0x8000, OFPXMT_OFB_ARP_THA, 6)
var OXM_OF_IPV6_SRC = oxmHeader(0x8000, OFPXMT_OFB_IPV6_SRC, 16)
var OXM_OF_IPV6_SRC_W = oxmHeaderW(0x8000, OFPXMT_OFB_IPV6_SRC, 16)
var OXM_OF_IPV6_DST = oxmHeader(0x8000, OFPXMT_OFB_IPV6_DST, 16)
var OXM_OF_IPV6_DST_W = oxmHeaderW(0x8000, OFPXMT_OFB_IPV6_DST, 16)
var OXM_OF_IPV6_FLABEL = oxmHeader(0x8000, OFPXMT_OFB_IPV6_FLABEL, 4)
var OXM_OF_IPV6_FLABEL_W = oxmHeaderW(0x8000, OFPXMT_OFB_IPV6_FLABEL, 4)
var OXM_OF_ICMPV6_TYPE = oxmHeader(0x8000, OFPXMT_OFB_ICMPV6_TYPE, 1)
var OXM_OF_ICMPV6_CODE = oxmHeader(0x8000, OFPXMT_OFB_ICMPV6_CODE, 1)
var OXM_OF_IPV6_ND_TARGET = oxmHeader(0x8000, OFPXMT_OFB_IPV6_ND_TARGET, 16)
var OXM_OF_IPV6_ND_SLL = oxmHeader(0x8000, OFPXMT_OFB_IPV6_ND_SLL, 6)
var OXM_OF_IPV6_ND_TLL = oxmHeader(0x8000, OFPXMT_OFB_IPV6_ND_TLL, 6)
var OXM_OF_MPLS_LABEL = oxmHeader(0x8000, OFPXMT_OFB_MPLS_LABEL, 4)
var OXM_OF_MPLS_TC = oxmHeader(0x8000, OFPXMT_OFB_MPLS_TC, 1)
var OXM_OF_MPLS_BOS = oxmHeader(0x8000, OFPXMT_OFB_MPLS_BOS, 1)
var OXM_OF_PBB_ISID = oxmHeader(0x8000, OFPXMT_OFB_PBB_ISID, 3)
var OXM_OF_PBB_ISID_W = oxmHeaderW(0x8000, OFPXMT_OFB_PBB_ISID, 3)
var OXM_OF_TUNNEL_ID = oxmHeader(0x8000, OFPXMT_OFB_TUNNEL_ID, 8)
var OXM_OF_TUNNEL_ID_W = oxmHeaderW(0x8000, OFPXMT_OFB_TUNNEL_ID, 8)
var OXM_OF_IPV6_EXTHDR = oxmHeader(0x8000, OFPXMT_OFB_IPV6_EXTHDR, 2)
var OXM_OF_IPV6_EXTHDR_W = oxmHeaderW(0x8000, OFPXMT_OFB_IPV6_EXTHDR, 2)

// ofp_ipv6exthdr_flags
const (
	OFPIEH_NONEXT = 1 << 0
	OFPIEH_ESP    = 1 << 1
	OFPIEH_AUTH   = 1 << 2
	OFPIEH_DEST   = 1 << 3
	OFPIEH_FRAG   = 1 << 4
	OFPIEH_ROUTER = 1 << 5
	OFPIEH_HOP    = 1 << 6
	OFPIEH_UNREP  = 1 << 7
	OFPIEH_UNSEQ  = 1 << 8
)

// ofp_action_type
const (
	OFPAT_OUTPUT       = 0
	OFPAT_COPY_TTL_OUT = 11
	OFPAT_COPY_TTL_IN  = 12
	OFPAT_SET_MPLS_TTL = 15
	OFPAT_DEC_MPLS_TTL = 16
	OFPAT_PUSH_VLAN    = 17
	OFPAT_POP_VLAN     = 18
	OFPAT_PUSH_MPLS    = 19
	OFPAT_POP_MPLS     = 20
	OFPAT_SET_QUEUE    = 21
	OFPAT_GROUP        = 22
	OFPAT_SET_NW_TTL   = 23
	OFPAT_DEC_NW_TTL   = 24
	OFPAT_SET_FIELD    = 25
	OFPAT_PUSH_PBB     = 26
	OFPAT_POP_PBB      = 27
	OFPAT_EXPERIMENTER = 0xffff
)

const OFP_NO_BUFFER = 0xffffffff

// ofp_controller_max_len
const (
	OFPCML_MAX       = 0xffe5
	OFPCML_NO_BUFFER = 0xffff
)

// ofp_instruction_type
const (
	OFPIT_GOTO_TABLE     = 1
	OFPIT_WRITE_METADATA = 2
	OFPIT_WRITE_ACTIONS  = 3
	OFPIT_APPLY_ACTIONS  = 4
	OFPIT_CLEAR_ACTIONS  = 5
	OFPIT_METER          = 6
	OFPIT_EXPERIMENTER   = 0xffff
)

const (
	OFPFC_ADD = iota
	OFPFC_MODIFY
	OFPFC_MODIFY_STRICT

	OFPFC_DELETE
	OFPFC_DELETE_STRICT
)

const (
	OFP_FLOW_PERMANENT = 0
)

const (
	OFP_DEFAULT_PRIORITY = 0x8000
)

// ofp_flow_mod_flags
const (
	OFPFF_SEND_FLOW_REM = 1 << 0
	OFPFF_CHECK_OVERLAP = 1 << 1
	OFPFF_RESET_COUNTS  = 1 << 2
	OFPFF_NO_PKT_COUNTS = 1 << 3
	OFPFF_NO_BYT_COUNTS = 1 << 4
)

// ofp_group
const (
	OFPG_MAX = 0xffffff00
	OFPG_ALL = 0xfffffffc
	OFPG_ANY = 0xffffffff
)

// ofp_group_mod_command
const (
	OFPGC_ADD = iota
	OFPGC_MODIFY
	OFPGC_DELETE
)

// ofp_group_type
const (
	OFPGT_ALL = iota
	OFPGT_SELECT
	OFPGT_INDIRECT
	OFPGT_FF
)

// ofp_packet_in_reason
const (
	OFPR_NO_MATCH = iota
	OFPR_ACTION
	OFPR_INVALID_TTL
)

// ofp_flow_removed_reason
const (
	OFPRR_IDLE_TIMEOUT = iota
	OFPRR_HARD_TIMEOUT
	OFPRR_DELETE
	OFPRR_GROUP_DELETE
)

// ofp_meter
const (
	OFPM_MAX        = 0xffff0000
	OFPM_SLOWPATH   = 0xfffffffd
	OFPM_CONTROLLER = 0xfffffffe
	OFPM_ALL        = 0xffffffff
)

// ofp_meter_band_type
const (
	OFPMBT_DROP         = 1
	OFPMBT_DSCP_REMARK  = 2
	OFPMBT_EXPERIMENTER = 0xFFFF
)

/* Meter Commands */

//ofp_meter_mod_command
const (
	OFPMC_ADD = iota
	OFPMC_MODIFY
	OFPMC_DELETE
)

// ofp_meter_flags
const (
	OFPMF_KBPS  = 1 << 0
	OFPMF_PKTPS = 1 << 1
	OFPMF_BURST = 1 << 2
	OFPMF_STATS = 1 << 3
)

// ofp_error_type
const (
	OFPET_HELLO_FAILED = iota
	OFPET_BAD_REQUEST
	OFPET_BAD_ACTION
	OFPET_BAD_INSTRUCTION
	OFPET_BAD_MATCH
	OFPET_FLOW_MOD_FAILED
	OFPET_GROUP_MOD_FAILED
	OFPET_PORT_MOD_FAILED
	OFPET_TABLE_MOD_FAILED
	OFPET_QUEUE_OP_FAILED
	OFPET_SWITCH_CONFIG_FAILED
	OFPET_ROLE_REQUEST_FAILED
	OFPET_METER_MOD_FAILED
	OFPET_TABLE_FEATURES_FAILED
	OFPET_EXPERIMENTER = 0xffff
)

// ofp_hello_failed_code
const (
	OFPHFC_INCOMPATIBLE = iota
	OFPHFC_EPERM
)

// ofp_bad_request_code
const (
	OFPBRC_BAD_VERSION = iota
	OFPBRC_BAD_TYPE
	OFPBRC_BAD_MULTIPART
	OFPBRC_BAD_EXPERIMENTER
	OFPBRC_BAD_EXP_TYPE
	OFPBRC_EPERM
	OFPBRC_BAD_LEN
	OFPBRC_BUFFER_EMPTY
	OFPBRC_BUFFER_UNKNOWN
	OFPBRC_BAD_TABLE_ID
	OFPBRC_IS_SLAVE
	OFPBRC_BAD_PORT
	OFPBRC_BAD_PACKET
	OFPBRC_MULTIPART_BUFFER_OVERFLOW
)

// ofp_bad_action_code
const (
	OFPBAC_BAD_TYPE = iota
	OFPBAC_BAD_LEN
	OFPBAC_BAD_EXPERIMENTER
	OFPBAC_BAD_EXP_TYPE
	OFPBAC_BAD_OUT_PORT
	OFPBAC_BAD_ARGUMENT
	OFPBAC_EPERM
	OFPBAC_TOO_MANY
	OFPBAC_BAD_QUEUE
	OFPBAC_BAD_OUT_GROUP
	OFPBAC_MATCH_INCONSISTENT
	OFPBAC_UNSUPPORTED_ORDER
	OFPBAC_BAD_TAG
	OFPBAC_BAD_SET_TYPE
	OFPBAC_BAD_SET_LEN
	OFPBAC_BAD_SET_ARGUMENT
)

// ofp_bad_instruction_code
const (
	OFPBIC_UNKNOWN_INST = iota
	OFPBIC_UNSUP_INST
	OFPBIC_BAD_TABLE_ID
	OFPBIC_UNSUP_METADATA
	OFPBIC_UNSUP_METADATA_MASK
	OFPBIC_BAD_EXPERIMENTER
	OFPBIC_BAD_EXP_TYPE
	OFPBIC_BAD_LEN
	OFPBIC_EPERM
)

// ofp_bad_match_code
const (
	OFPBMC_BAD_TYPE = iota
	OFPBMC_BAD_LEN
	OFPBMC_BAD_TAG
	OFPBMC_BAD_DL_ADDR_MASK
	OFPBMC_BAD_NW_ADDR_MASK
	OFPBMC_BAD_WILDCARDS
	OFPBMC_BAD_FIELD
	OFPBMC_BAD_VALUE
	OFPBMC_BAD_MASK
	OFPBMC_BAD_PREREQ
	OFPBMC_DUP_FIELD
	OFPBMC_EPERM
)

// ofp_flow_mod_failed_code
const (
	OFPFMFC_UNKNOWN = iota
	OFPFMFC_TABLE_FULL
	OFPFMFC_BAD_TABLE_ID
	OFPFMFC_OVERLAP
	OFPFMFC_EPERM
	OFPFMFC_BAD_TIMEOUT
	OFPFMFC_BAD_COMMAND
	OFPFMFC_BAD_FLAGS
)

// ofp_group_mod_failed_code
const (
	OFPGMFC_GROUP_EXISTS = iota
	OFPGMFC_INVALID_GROUP
	OFPGMFC_WEIGHT_UNSUPPORTED
	OFPGMFC_OUT_OF_GROUPS
	OFPGMFC_OUT_OF_BUCKETS
	OFPGMFC_CHAINING_UNSUPPORTED
	OFPGMFC_WATCH_UNSUPPORTED
	OFPGMFC_LOOP
	OFPGMFC_UNKNOWN_GROUP
	OFPGMFC_CHAINED_GROUP
	OFPGMFC_BAD_TYPE
	OFPGMFC_BAD_COMMAND
	OFPGMFC_BAD_BUCKET
	OFPGMFC_BAD_WATCH
	OFPGMFC_EPERM
)

// ofp_port_mod_failed_code
const (
	OFPPMFC_BAD_PORT = iota
	OFPPMFC_BAD_HW_ADDR
	OFPPMFC_BAD_CONFIG
	OFPPMFC_BAD_ADVERTISE
	OFPPMFC_EPERM
)

// ofp_table_mod_faled_code
const (
	OFPTMFC_BAD_TABLE = iota
	OFPTMFC_BAD_CONFIG
	OFPTMFC_EPERM
)

// ofp_queue_op_failed_code
const (
	OFPQOFC_BAD_PORT = iota
	OFPQOFC_BAD_QUEUE
	OFPQOFC_EPERM
)

// ofp_switch_config_failed_code
const (
	OFPSCFC_BAD_FLAGS = iota
	OFPSCFC_BAD_LEN
	OFPSCFC_EPERM
)

// ofp_role_request_failed_code
const (
	OFPRRFC_STALE = iota
	OFPRRFC_UNSUP
	OFPRRFC_BAD_ROLE
)

// ofp_meter_mod_failed_code
const (
	OFPMMFC_UNKNOWN = iota
	OFPMMFC_METER_EXISTS
	OFPMMFC_INVALID_METER
	OFPMMFC_UNKNOWN_METER
	OFPMMFC_BAD_COMMAND
	OFPMMFC_BAD_FLAGS
	OFPMMFC_BAD_RATE
	OFPMMFC_BAD_BURST
	OFPMMFC_BAD_BAND
	OFPMMFC_BAD_BAND_VALUE
	OFPMMFC_OUT_OF_METERS
	OFPMMFC_OUT_OF_BANDS
)

// ofp_table_features_failed_code
const (
	OFPTFFC_BAD_TABLE = iota
	OFPTFFC_BAD_METADATA
	OFPTFFC_BAD_TYPE
	OFPTFFC_BAD_LEN
	OFPTFFC_BAD_ARGUMENT
	OFPTFFC_EPERM
)

// ofp_multipart_type
const (
	OFPMP_DESC = iota
	OFPMP_FLOW
	OFPMP_AGGREGATE
	OFPMP_TABLE
	OFPMP_PORT_STATS
	OFPMP_QUEUE
	OFPMP_GROUP
	OFPMP_GROUP_DESC
	OFPMP_GROUP_FEATURES
	OFPMP_METER
	OFPMP_METER_CONFIG
	OFPMP_METER_FEATURES
	OFPMP_TABLE_FEATURES
	OFPMP_PORT_DESC
	OFPMP_EXPERIMENTER = 0xffff
)

// ofp_multipart_request_flags
const (
	OFPMPF_REQ_MORE = 1 << 0
)

// ofp_multipart_reply_flags
const (
	OFPMPF_REPLY_MORE = 1 << 0
)

const DESC_STR_LEN = 256
const SERIAL_NUM_LEN = 32

// ofp_table_feature_prop_type
const (
	OFPTFPT_INSTRUCTIONS        = 0
	OFPTFPT_INSTRUCTIONS_MISS   = 1
	OFPTFPT_NEXT_TABLES         = 2
	OFPTFPT_NEXT_TABLES_MISS    = 3
	OFPTFPT_WRITE_ACTIONS       = 4
	OFPTFPT_WRITE_ACTIONS_MISS  = 5
	OFPTFPT_APPLY_ACTIONS       = 6
	OFPTFPT_APPLY_ACTIONS_MISS  = 7
	OFPTFPT_MATCH               = 8
	OFPTFPT_WILDCARDS           = 10
	OFPTFPT_WRITE_SETFIELD      = 12
	OFPTFPT_WRITE_SETFIELD_MISS = 13
	OFPTFPT_APPLY_SETFIELD      = 14
	OFPTFPT_APPLY_SETFIELD_MISS = 15
	OFPTFPT_EXPERIMENTER        = 0xfffe
	OFPTFPT_EXPERIMENTER_MISS   = 0xffff
)

// ofp_group_capabilities
const (
	OFPGC_SELECT_WEIGHT   = 1 << 0
	OFPGC_SELECT_LIVENESS = 1 << 1
	OFPGC_CHAINING        = 1 << 2
	OFPGC_CHAINING_CHECKS = 1 << 3
)

const OFPQ_ALL = 0xffffffff

const OFPQ_MIN_RATE_UNCFG = 0xffff

const OFPQ_MAX_RATE_UNCFG = 0xffff

// ofp_queue_properties
const (
	OFPQT_MIN_RATE     = 1
	OFPQT_MAX_RATE     = 2
	OFPQT_EXPERIMENTER = 0xffff
)

// ofp_controller_role
const (
	OFPCR_ROLE_NOCHANGE = 0
	OFPCR_ROLE_EQUAL    = 1
	OFPCT_ROLE_MASTER   = 2
	OFPCR_ROLE_SLAVE    = 3
)

/**
 * struct
 */

/**
 * OfpHeader
 */
type OfpHeader struct {
	Version uint8
	Type    uint8
	Length  uint16
	Xid     uint32
}

type OfpHelloElemHeader struct {
	Type   uint16
	Length uint16
}

type OfpHelloElemVersionBitmap struct {
	Type    uint16
	Length  uint16
	Bitmaps []uint32
}

/**
 * Hello Message
 */
type OfpHello struct {
	Header   OfpHeader
	Elements []OfpHelloElemHeader
}

type OfpSwitchConfig struct {
	Header      OfpHeader
	Flags       uint16
	MissSendLen uint16
}

type OfpTableMod struct {
	Header  OfpHeader
	TableId uint8
	Pad     [3]uint8
	Config  uint32
}

type OfpPort struct {
	PortNo uint32
	// Pad    [4]uint8
	HwAddr net.HardwareAddr
	// Pad2   [2]uint8
	Name       []byte // 16
	Config     uint32
	State      uint32
	Curr       uint32
	Advertised uint32
	Supported  uint32
	Peer       uint32
	CurrSpeed  uint32
	MaxSpeed   uint32
}

type OfpSwitchFeatures struct {
	Header       OfpHeader
	DatapathId   uint64
	NBuffers     uint32
	NTables      uint8
	AuxiliaryId  uint8
	Pad          [2]uint8
	Capabilities uint32
	Reserved     uint32
}

type OfpPortStatus struct {
	Header OfpHeader
	Reason uint8
	// Pad    [7]uint8
	Desc *OfpPort
}

type OfpPortMod struct {
	Header OfpHeader
	PortNo uint32
	// Pad       [4]uint8
	HwAddr net.HardwareAddr // 6
	// Pad2      [2]uint8
	Config    uint32
	Mask      uint32
	Advertise uint32
	// pad3      [4]uint8
}

type OfpMatch struct {
	Type      uint16
	Length    uint16
	OxmFields []OxmField
	// Pad       []uint8
}

type OxmField interface {
	Serialize() []byte
	Parse([]byte)
	OxmClass() uint32
	OxmField() uint32
	OxmHasMask() uint32
	Length() uint32
	Size() int
}

/*
 * MatchField definition
 */

type OxmInPort struct {
	TlvHeader uint32
	Value     uint32
}

type OxmInPhyPort struct {
	TlvHeader uint32
	Value     uint32
}

type OxmMetadata struct {
	TlvHeader uint32
	Value     uint64
	Mask      uint64
}

type OxmEth struct {
	TlvHeader uint32
	Value     net.HardwareAddr
	Mask      net.HardwareAddr
}

type OxmEthType struct {
	TlvHeader uint32
	Value     uint16
}

type OxmVlanVid struct {
	TlvHeader uint32
	Value     uint16
	Mask      uint16
}

type OxmVlanPcp struct {
	TlvHeader uint32
	Value     uint8
}

type OxmIpDscp struct {
	TlvHeader uint32
	Value     uint8
}

type OxmIpEcn struct {
	TlvHeader uint32
	Value     uint8
}

type OxmIpProto struct {
	TlvHeader uint32
	Value     uint8
}

type OxmIpv4 struct {
	TlvHeader uint32
	Value     net.IP
	Mask      net.IPMask
}

type OxmTcp struct {
	TlvHeader uint32
	Value     uint16
}

type OxmUdp struct {
	TlvHeader uint32
	Value     uint16
}

type OxmSctp struct {
	TlvHeader uint32
	Value     uint16
}

type OxmIcmpType struct {
	TlvHeader uint32
	Value     uint8
}

type OxmIcmpCode struct {
	TlvHeader uint32
	Value     uint8
}

type OxmArpOp struct {
	TlvHeader uint32
	Value     uint16
}

type OxmArpPa struct {
	TlvHeader uint32
	Value     net.IP
	Mask      net.IPMask
}

type OxmArpHa struct {
	TlvHeader uint32
	//Value     [6]uint8
	Value net.HardwareAddr
}

type OxmIpv6 struct {
	TlvHeader uint32
	Value     net.IP
	Mask      net.IPMask
}

type OxmIpv6FLabel struct {
	TlvHeader uint32
	Value     uint32
	Mask      uint32
}

type OxmIcmpv6Type struct {
	TlvHeader uint32
	Value     uint8
}

type OxmIcmpv6Code struct {
	TlvHeader uint32
	Value     uint8
}

type OxmIpv6NdTarget struct {
	TlvHeader uint32
	Value     net.IP
}

type OxmIpv6NdSll struct {
	TlvHeader uint32
	// Value     [6]uint8
	Value net.HardwareAddr
}

type OxmIpv6NdTll struct {
	TlvHeader uint32
	// Value     [6]uint8
	Value net.HardwareAddr
}

type OxmMplsLabel struct {
	TlvHeader uint32
	Value     uint32
}

type OxmMplsTc struct {
	TlvHeader uint32
	Value     uint8
}

type OxmMplsBos struct {
	TlvHeader uint32
	Value     uint8
}

type OxmPbbIsid struct {
	TlvHeader uint32
	Value     [3]uint8
	Mask      [3]uint8
}

type OxmTunnelId struct {
	TlvHeader uint32
	Value     uint64
	Mask      uint64
}

type OxmIpv6ExtHeader struct {
	TlvHeader uint32
	Value     uint16
	Mask      uint16
}

type OfpOxmExperimenterHeader struct {
	OxmHeader    uint32
	Experimenter uint32
}

/**
 * Actions
 */

type OfpAction interface {
	Serialize() []byte
	Parse(packet []byte)
	Size() int
	OfpActionType() uint16
}

type OfpActionHeader struct {
	Type   uint16
	Length uint16
}

type OfpActionOutput struct {
	ActionHeader OfpActionHeader
	Port         uint32
	MaxLen       uint16
	Pad          [6]uint8
}

type OfpActionCopyTtlOut struct {
	ActionHeader OfpActionHeader
	Pad          [4]uint8
}

type OfpActionCopyTtlIn struct {
	ActionHeader OfpActionHeader
	Pad          [4]uint8
}

type OfpActionSetMplsTtl struct {
	ActionHeader OfpActionHeader
	MplsTtl      uint8
	Pad          [3]uint8
}

type OfpActionDecMplsTtl struct {
	ActionHeader OfpActionHeader
	Pad          [4]uint8
}

type OfpActionPush struct {
	ActionHeader OfpActionHeader
	EtherType    uint16
	Pad          [2]uint8
}

type OfpActionPop struct {
	ActionHeader OfpActionHeader
	EtherType    uint16
	Pad          [2]uint8
}

type OfpActionGroup struct {
	ActionHeader OfpActionHeader
	GroupId      uint32
}

type OfpActionSetQueue struct {
	ActionHeader OfpActionHeader
	QueueId      uint32
}
type OfpActionSetNwTtl struct {
	ActionHeader OfpActionHeader
	NwTtl        uint8
	Pad          [3]uint8
}

type OfpActionDecNwTtl struct {
	ActionHeader OfpActionHeader
	Pad          [4]uint8
}

type OfpActionSetField struct {
	ActionHeader OfpActionHeader
	Oxm          OxmField
	//Field        [4]uint8
}

type OfpActionExperimenter struct {
	ActionHeader OfpActionHeader
	Experimenter uint32
}

/**
 * Instructions
 */

type OfpInstruction interface {
	Serialize() []byte
	Parse(packet []byte)
	Size() int
	InstructionType() uint16
}

type OfpInstructionHeader struct {
	Type   uint16
	Length uint16
}

type OfpInstructionGotoTable struct {
	Header  OfpInstructionHeader
	TableId uint8
	Pad     [3]uint8
}

type OfpInstructionWriteMetadata struct {
	Header       OfpInstructionHeader
	Pad          [4]uint8
	Metadata     uint64
	MetadataMask uint64
}

type OfpInstructionActions struct {
	Header  OfpInstructionHeader
	Pad     [4]uint8
	Actions []OfpAction
}

type OfpInstructionMeter struct {
	Header  OfpInstructionHeader
	MeterId uint32
}

type OfpInstructionExperimenter struct {
	Header       OfpInstructionHeader
	Experimenter uint32
}

type OfpFlowMod struct {
	Header      OfpHeader
	Cookie      uint64
	CookieMask  uint64
	TableId     uint8
	Command     uint8
	IdleTimeout uint16
	HardTimeout uint16
	Priority    uint16
	BufferId    uint32
	OutPort     uint32
	OutGroup    uint32
	Flags       uint16
	// Pad          [2]uint8 // 2
	Match        *OfpMatch
	Instructions []OfpInstruction
}

type OfpBucket struct {
	Length     uint16
	Weight     uint16
	WatchPort  uint32
	WatchGroup uint32
	Pad        [4]uint8
	Actions    []OfpAction
}

type OfpGroupMod struct {
	Header  OfpHeader
	Command uint16
	Type    uint8
	Pad     uint8
	GroupId uint32
	Buckets []*OfpBucket
}

type OfpPacketOut struct {
	Header    OfpHeader
	BufferId  uint32
	InPort    uint32
	ActionLen uint16
	Pad       [6]uint8
	Actions   []OfpAction
	Data      []byte
}

type OfpPacketIn struct {
	Header   OfpHeader
	BufferId uint32
	TotalLen uint16
	Reason   uint8
	TableId  uint8
	Cookie   uint64
	Match    *OfpMatch
	Pad      [2]uint8
	Data     []uint8
}

type OfpFlowRemoved struct {
	Header       OfpHeader
	Cookie       uint64
	Priority     uint16
	Reason       uint8
	TableId      uint8
	DurationSec  uint32
	DurationNSec uint32
	IdleTimeout  uint16
	HardTimeout  uint16
	PacketCount  uint64
	ByteCount    uint64
	Match        *OfpMatch
}

type OfpMeterBand interface {
	Serialize() []byte
	Parse(packet []byte)
	Size() int
	MeterBandType() uint16
}

type OfpMeterBandHeader struct {
	Type      uint16
	Length    uint16
	Rate      uint32
	BurstSize uint32
}

type OfpMeterBandDrop struct {
	Header OfpMeterBandHeader
	// Pad    [4]uint8
}

type OfpMeterBandDscpRemark struct {
	Header    OfpMeterBandHeader
	PrecLevel uint8
	// Pad       [3]uint8
}

type OfpMeterBandExperimenter struct {
	Header       OfpMeterBandHeader
	Experimenter uint32
}

type OfpMeterMod struct {
	Header  OfpHeader
	Command uint16
	Flags   uint16
	MeterId uint32
	Bands   []OfpMeterBand
}

type OfpErrorMsg struct {
	Header OfpHeader
	Type   uint16
	Code   uint16
	Data   []uint8
}

type OfpErrorExperimenterMsg struct {
	Header       OfpHeader
	Type         uint16
	ExpType      uint16
	Experimenter uint32
	Data         []uint8
}

type OfpMultipartBody interface {
	Serialize() []byte
	Parse(packet []byte)
	Size() int
	MPType() uint16
}

type OfpMultipartRequest struct {
	Header OfpHeader
	Type   uint16
	Flags  uint16
	// Pad    [4]uint8
	Body OfpMultipartBody
}

type OfpMultipartReply struct {
	Header OfpHeader
	Type   uint16
	Flags  uint16
	// Pad    [4]uint8
	Body []OfpMultipartBody
}

type OfpDescStats struct {
	MfrDesc   []uint8
	HwDesc    []uint8
	SwDesc    []uint8
	SerialNum []uint8
	DpDesc    []uint8
}

type OfpFlowStatsRequest struct {
	TableId uint8
	// Pad        [3]uint8
	OutPort  uint32
	OutGroup uint32
	// Pad2       [4]uint8
	Cookie     uint64
	CookieMask uint64
	Match      *OfpMatch
}

type OfpFlowStats struct {
	Length       uint16
	TableId      uint8
	Pad          uint8
	DurationSec  uint32
	DurationNSec uint32
	Priority     uint16
	IdleTimeout  uint16
	HardTimeout  uint16
	Flags        uint16
	// Pad2         [4]uint8
	Cookie       uint64
	PacketCount  uint64
	ByteCount    uint64
	Match        *OfpMatch
	Instructions []OfpInstruction
}

type OfpAggregateStatsRequest struct {
	TableId uint8
	// Pad        [3]uint8
	OutPort  uint32
	OutGroup uint32
	// Pad2       [4]uint8
	Cookie     uint64
	CookieMask uint64
	Match      *OfpMatch
}

type OfpAggregateStats struct {
	PacketCount uint64
	ByteCount   uint64
	FlowCount   uint32
	// Pad         [4]uint8
}

type OfpTableFeatureProp interface {
	Serialize() []byte
	Parse(packet []byte)
	Size() int
	Property() uint16
}

type OfpTableFeaturePropHeader struct {
	Type   uint16
	Length uint16
}

type OfpInstructionId struct {
	Type   uint16
	Length uint16
}

type OfpTableFeaturePropInstructions struct {
	PropHeader     OfpTableFeaturePropHeader
	InstructionIds []*OfpInstructionId
}

type OfpTableFeaturePropNextTables struct {
	PropHeader   OfpTableFeaturePropHeader
	NextTableIds []uint8
}

type OfpTableFeaturePropActions struct {
	PropHeader OfpTableFeaturePropHeader
	ActionIds  []OfpActionHeader
}

type OfpTableFeaturePropOxm struct {
	PropHeader OfpTableFeaturePropHeader
	OxmIds     []uint32
}

type OfpTableFeaturePropExperimenter struct {
	PropHeader       OfpTableFeaturePropHeader
	Experimenter     uint32
	ExpType          uint32
	ExperimenterData []uint32
}

type OfpTableFeatures struct {
	Length  uint16
	TableId uint8
	// Pad           [5]uint8
	Name          []byte
	MetadataMatch uint64
	MetadataWrite uint64
	Config        uint32
	MaxEntries    uint32
	Properties    []OfpTableFeatureProp
}

type OfpTableStats struct {
	TableId      uint8
	Pad          [3]uint8
	ActiveCount  uint32
	LookupCount  uint64
	MatchedCount uint64
}

type OfpPortStatsRequest struct {
	PortNo uint32
	// Pad    [4]uint8
}

type OfpPortStats struct {
	PortNo uint32
	// Pad          [4]uint8
	RxPackets    uint64
	TxPackets    uint64
	RxBytes      uint64
	TxBytes      uint64
	RxDropped    uint64
	TxDropped    uint64
	RxErrors     uint64
	TxErrors     uint64
	RxFrameErr   uint64
	RxOverErr    uint64
	RxCrcErr     uint64
	Collisions   uint64
	DurationSec  uint32
	DurationNSec uint32
}

type OfpQueueStatsRequest struct {
	PortNo  uint32
	QueueId uint32
}

type OfpQueueStats struct {
	PortNo       uint32
	QueueId      uint32
	TxBytes      uint64
	TxPackets    uint64
	TxErrors     uint64
	DurationSec  uint32
	DurationNSec uint32
}

type OfpGroupStatsRequest struct {
	GroupId uint32
	Pad     [4]uint8
}

type OfpBucketCounter struct {
	PacketCount uint64
	ByteCount   uint64
}

type OfpGroupStats struct {
	Length uint16
	// Pad          [2]uint8
	GroupId  uint32
	RefCount uint32
	// Pad2         [4]uint8
	PacketCount  uint64
	ByteCount    uint64
	DurationSec  uint32
	DurationNSec uint32
	BucketStats  []*OfpBucketCounter
}

type OfpGroupDescStats struct {
	Length  uint16
	Type    uint8
	Pad     uint8
	GroupId uint32
	Buckets []*OfpBucket
}

type OfpGroupFeaturesStats struct {
	Type         uint32
	Capabilities uint32
	MaxGroups    [4]uint32
	Actions      [4]uint32
}

type OfpMeterMultipartRequest struct {
	MeterId uint32
	// Pad     [4]uint8
}

type OfpMeterBandStats struct {
	PacketBandCount uint64
	ByteBandCount   uint64
}

type OfpMeterStats struct {
	MeterId uint32
	Length  uint16
	// Pad           [6]uint8
	FlowCount     uint32
	PacketInCount uint64
	ByteInCount   uint64
	DurationSec   uint32
	DurationNSec  uint32
	BandStats     []*OfpMeterBandStats
}

type OfpMeterConfig struct {
	Length  uint16
	Flags   uint16
	MeterId uint32
	Bands   []OfpMeterBand
}

type OfpMeterFeatures struct {
	MaxMeter     uint32
	BandTypes    uint32
	Capabilities uint32
	MaxBands     uint8
	MaxColor     uint8
	Pad          [2]uint8
}

type OfpExperimenterMultipartHeader struct {
	Header       OfpHeader
	Experimenter uint32
	ExpType      uint32
}

type OfpQueueProp interface {
	Parse(packet []byte)
	Size() int
	Property() uint16
}

type OfpQueuePropHeader struct {
	Property uint16
	Length   uint16
	// Pad      [4]uint8
}

type OfpQueuePropMinRate struct {
	PropHeader OfpQueuePropHeader
	Rate       uint16
	// Pad        [6]uint8
}

type OfpQueuePropMaxRate struct {
	PropHeader OfpQueuePropHeader
	Rate       uint16
	// Pad        [6]uint8
}

type OfpQueuePropExperimenter struct {
	PropHeader   OfpQueuePropHeader
	Experimenter uint32
	// Pad          [4]uint8
	Data []uint8
}

type OfpPacketQueue struct {
	QueueId uint32
	Port    uint32
	Length  uint16
	// Pad        [6]uint8
	Properties []OfpQueueProp
}

type OfpQueueGetConfigRequest struct {
	Header OfpHeader
	Port   uint32
	// Pad    [4]uint8
}

type OfpQueueGetConfigReply struct {
	Header OfpHeader
	Port   uint32
	// Pad    [4]uint8
	Queue []*OfpPacketQueue
}

type OfpRole struct {
	Header OfpHeader
	Role   uint32
	// Pad          [4]uint8
	GenerationId uint64
}

type OfpAsyncConfig struct {
	Header          OfpHeader
	PacketInMask    [2]uint32
	PortStatusMask  [2]uint32
	FlowRemovedMask [2]uint32
}
