package openflow

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/big"
	"net"
	"strings"
)

func macToHex(mac string) string {
	return fmt.Sprintf("0x%s", strings.Replace(mac, ":", "", -1))
}
func ipToHex(ip4Address string) string {
	ipv4Decimal := IP4toInt(net.ParseIP(ip4Address))

	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, uint32(ipv4Decimal))

	if err != nil {
		fmt.Println("Unable to write to buffer:", err)
	}

	// present in hexadecimal format
	result := "0x" +strings.TrimPrefix(fmt.Sprintf("%x", buf.Bytes()), "0")
	return result
}


func IP4toInt(IPv4Address net.IP) int64 {
	IPv4Int := big.NewInt(0)
	IPv4Int.SetBytes(IPv4Address.To4())
	return IPv4Int.Int64()
}

