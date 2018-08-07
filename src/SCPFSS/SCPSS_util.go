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
	"sync"
	"time"
)

type Greet struct {
	Hello string
}

type FileHash struct {
	FileCheckSum string
}

type SCPFSFileInfo struct {
	Name    string
	Path    string
	Size    int64
	LastMod time.Time
}

type fileMapper struct {
	hashToFileInfo map[string]SCPFSFileInfo
	filePathToHash map[string]string
	idToFilePath   []string
}

type serverList struct {
	list   []string
	length int32
}

type ProgressBar struct {
	Percent        uint8
	UpdateInterval uint8
	Title          string
	Right          string
	wg             *sync.WaitGroup
	StopSig        bool
	IfTitle        bool
	IfRight        bool
}

const (
	LOG_INFO  uint8 = 1
	LOG_ERROR uint8 = 2
)

func (info *SCPFSFileInfo) Print() {
	fmt.Printf(info.Name + "\t\t" + strconv.Itoa(int(info.Size)) + "\t\t" + info.LastMod.Format("2006/01/02 15:04:05") + "\n")
}

func PrintLog(log string, logType uint8) {
	s := "[" + time.Unix(time.Now().Unix(), 0).Format("2006/01/02 15:04:05") + "]" + log + "\n"
	if logType == LOG_INFO {
		s = "[INFO]" + s
	} else if logType == LOG_ERROR {
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
		t1, err1 := net.Listen("tcp", ":"+strconv.Itoa(int(st)+1))
		if err != nil || err1 != nil {
			if t != nil {
				t.Close()
			}
			if t1 != nil {
				t1.Close()
			}
			st += 1
		} else {
			t.Close()
			t1.Close()
			break
		}
	}
	return st
}

func NewProgressBar(interval uint8, t, r bool, title string) *ProgressBar {
	ret := new(ProgressBar)
	ret.IfRight = r
	ret.IfTitle = t
	ret.UpdateInterval = interval
	ret.Title = title
	return ret
}

func (p *ProgressBar) Show() {
	if p.IfTitle {
		fmt.Println(p.Title)
	}
	fmt.Println("\n")
	pgb := "[" + strings.Repeat(" ", 50) + "]"
	if p.IfRight {
		fmt.Printf("%s %s\r", pgb, p.Right)
	} else {
		fmt.Printf("%s\r", pgb)
	}
	go p.Update()
}

func (p *ProgressBar) doUpdate() {
	pgb := "[" + strings.Repeat("=", int(p.Percent/2)) + strings.Repeat(" ", int(50-p.Percent/2)) + "]"
	if p.IfRight {
		fmt.Printf("%s %s\r", pgb, p.Right)
	} else {
		fmt.Printf("%s\r", pgb)
	}
}

func (p *ProgressBar) Update() {
	for !p.StopSig {
		p.doUpdate()
		time.Sleep(time.Duration(int64(p.UpdateInterval)) * time.Millisecond)
	}
}

func (p *ProgressBar) Stop() {
	p.StopSig = true
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
	if blockCount != 0 {
		for i := uint64(0); i < blockCount; i += 1 {
			bsz := int(math.Min(float64(fileChunk), float64(size-int64(i*uint64(fileChunk)))))
			buffer := make([]byte, bsz)
			file.Read(buffer)
			io.WriteString(sha1hash, string(buffer))
		}
	} else {
		bsz := info.Size()
		buffer := make([]byte, bsz)
		file.Read(buffer)
		io.WriteString(sha1hash, string(buffer))
	}
	rets := strings.ToUpper(fmt.Sprintf("%x", sha1hash.Sum(nil)))
	return rets, nil
}

func fillTo(raw string, tlen int32) string {
	for len(raw) < int(tlen) {
		raw += "/"
	}
	return raw
}
