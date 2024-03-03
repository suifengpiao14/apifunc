package apifunc

import (
	"github.com/suifengpiao14/stream/packet"
	"github.com/suifengpiao14/stream/packet/lineschemapacket"
)

/*********************************内置流程*****************************************/

func DefaultAPIFlows() (flows []string) {
	flows = []string{
		lineschemapacket.PACKETHANDLER_NAME_ValidatePacket,
		lineschemapacket.PACKETHANDLER_NAME_MergeDefaultPacketHandler,
		lineschemapacket.PACKETHANDLER_NAME_TransferTypeFormatPacket,
		packet.PACKETHANDLER_NAME_JsonAddTrimNamespacePacket,
		packet.PACKETHANDLER_NAME_TransferPacketHandler,
		PACKETHANDLER_NAME_API_LOGIC,
	}

	return flows
}

func DefaultTormFlows() (flows []string) {
	flows = []string{
		packet.PACKETHANDLER_NAME_CUDEvent,
		packet.PACKETHANDLER_NAME_MysqlPacketHandler,
	}
	return flows
}
