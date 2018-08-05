package SCPFSS

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	INFO  uint8 = 1
	ERROR uint8 = 2
)

func PrintLog(log string, logType uint8) {
	s := "[" + time.Unix(time.Now().Unix(), 0).Format("2006/01/02 15:04:05") + "]" + log + "\n"
	if logType == INFO {
		s = "[INFO]" + s
	} else if logType == ERROR {
		s = "[ERROR]" + s
	}
	fmt.Print(s)
}

func getIp() string {
	var ipAddress string
	addrList, err := net.InterfaceAddrs()
	if err != nil {
		panic("Fail to get IP address")
	}
	for _, a := range addrList {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ipAddress = ipnet.IP.String()
			}
		}
	}
	return ipAddress
}

func findFreePort(startPort int32) int32 {
	st := startPort
	for {
		t, err := net.Listen("tcp", ":"+strconv.Itoa(int(st)))
		if err != nil {
			st += 1
			t.Close()
		} else {
			t.Close()
			break
		}
	}
	return st
}

func sha1HashFile(filePath string) (string, error) {
	file, err := os.Open(filePath)
	sha1hash := sha1.New()
	if err != nil {
		rerr := errors.New("Hash fail when openfile")
		return "", rerr
	}
	defer file.Close()
	info, _ := file.Stat()
	size := info.Size()
	blockCount := uint64(math.Ceil(float64(size)) / float64(fileChunk))
	for i := uint64(0); i < blockCount; i += 1 {
		bsz := int(math.Min(float64(fileChunk), float64(size-int64(i*uint64(fileChunk)))))
		buffer := make([]byte, bsz)
		file.Read(buffer)
		io.WriteString(sha1hash, string(buffer))
	}
	rets := strings.ToUpper(fmt.Sprintf("%x", sha1hash.Sum(nil)))
	return rets, nil
}
