package SCPFSS

import (
	"bufio"
	"chordNode"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
)

const (
	welcomeInfo     string = "Welcome to use Stupid Chord Peer to peer File Sharing System"
	helpInfo        string = "There is no help info, you are on your own, you can choose to uninstall this stupid software."
	startShareInfo  string = "Share <YourFilePath>"
	stopShareInfo   string = "StopShare <YourFilePath>"
	findInfo        string = "Find <SCPFSS LINK>"
	joinInfo        string = "Join <IP:Port>"
	linkPrefix      string = "SCPFSP-SHA1:"
	defaultDhtPort  int32  = 1919
	defaultFilePort int32  = 2020
	fileChunk       int32  = 4096
)

type SCPFSS struct {
	server              *iSCPFSServer
	wg                  *sync.WaitGroup
	localDhtAddr        string
	localFileServerAddr string
}

func NewSCPFSS() *SCPFSS {
	ret := new(SCPFSS)
	ret.server = newSCPFSServer()
	port := findFreePort(defaultDhtPort)
	dht := chordNode.NewNode(port)
	ret.localDhtAddr = getIp() + ":" + strconv.Itoa(int(port))
	port = findFreePort(defaultFilePort)
	ret.localFileServerAddr = getIp() + ":" + strconv.Itoa(int(port))
	ret.wg = new(sync.WaitGroup)
	ret.wg.Add(1)
	go dht.Run(ret.wg)
	//TODO RUN FILE SERVER
	ret.server.dhtNode = dht
	return ret

}

func (sys *SCPFSS) Quit() {
	if sys.server.ifInNetwork == false || sys.server.dhtNode.InRing == false {
		return
	} else {
		for k, _ := range sys.server.fileShared.filePathToHash {
			sys.StopShare(k)
		}
		//TODO STOP FILE SERVER
		sys.server.dhtNode.Quit()
	}
}

func (sys *SCPFSS) Share(filePath string) (string, error) {
	if sys.server.ifInNetwork == false || sys.server.dhtNode.InRing == false {
		err := errors.New("Not in the SCPFS Network")
		return "", err
	}
	var v1, v2 string
	var ok1, ok2 bool
	v1, ok1 = sys.server.fileShared.filePathToHash[filePath]
	if ok1 {
		rerr := errors.New("File is still sharing")
		return linkPrefix + v1, rerr
	}
	fileCheckSum, rerr := sha1HashFile(filePath)
	if rerr != nil {
		return "", rerr
	}
	v2, ok2 = sys.server.fileShared.hashToFilePath[fileCheckSum]
	if ok2 {
		rerr := errors.New("File is still sharing")
		return linkPrefix + v2, rerr
	}
	sys.server.fileShared.idToFilePath = append(sys.server.fileShared.idToFilePath, fileCheckSum)
	sys.server.fileShared.filePathToHash[filePath] = fileCheckSum
	sys.server.fileShared.hashToFilePath[fileCheckSum] = filePath
	ret := sys.server.dhtNode.AppendToData(fileCheckSum, sys.localFileServerAddr+";")
	if ret == 2 {
		err := errors.New("This file has been shared before")
		return linkPrefix + fileCheckSum, err
	} else if ret == 1 {
		return linkPrefix + fileCheckSum, nil
	} else {
		return "", nil
	}
}

func (sys *SCPFSS) StopShare(filePath string) (bool, error) {
	if sys.server.ifInNetwork == false || sys.server.dhtNode.InRing == false {
		err := errors.New("Not in the SCPFS Network")
		return false, err
	}
	v1, ok1 := sys.server.fileShared.filePathToHash[filePath]
	if !ok1 {
		err := errors.New("File has not been shared before")
		return false, err
	}
	ret := sys.server.dhtNode.RemoveFromData(v1, sys.localFileServerAddr+";")
	if ret == 0 {
		err := errors.New("File remove failed for SCPFS reasons, please try again later")
		return false, err
	} else if ret == 3 {
		err := errors.New("File not found in SCPFS network")
		return false, err
	}
	return true, nil
}

func (sys *SCPFSS) JoinNetwork(addr string) (bool, error) {
	if !sys.server.dhtNode.Join(addr) {
		err := errors.New("Fail to join DHT network")
		return false, err
	}
	//TODO check if target ip has file serve
	sys.server.ifInNetwork = true
	return true, nil
}

func (sys *SCPFSS) CreateNetwork() (bool, error) {
	sys.server.dhtNode.Create()
	sys.server.ifInNetwork = true
	return true, nil
}

func (sys *SCPFSS) LookUpFile(link string) (bool, error) {
	hashid := strings.Replace(link, linkPrefix, "", -1)
	return true, nil
}

func (sys *SCPFSS) ListFileShared() string {
	return ""
}

func (sys *SCPFSS) handleCmd(cmd string) int {
	splitedCmd := strings.Fields(cmd)
	switch splitedCmd[0] {
	case "exit":
		sys.Quit()
		return 2
	case "share":
		if len(splitedCmd) == 2 {
			sys.Share(splitedCmd[1])
		} else {
			PrintLog(startShareInfo, INFO)
		}
		return 1
	case "stopshare":
		if len(splitedCmd) == 2 {
			sys.StopShare(splitedCmd[1])
		} else {
			PrintLog(stopShareInfo, INFO)
		}
		return 1
	case "join":
		if len(splitedCmd) == 2 {
			sys.JoinNetwork(splitedCmd[1])
		} else {
			PrintLog(joinInfo, INFO)
		}
		return 1
	case "find":
		if len(splitedCmd) == 2 {
			sys.LookUpFile(splitedCmd[1])
		} else {
			PrintLog(findInfo, INFO)
		}
		return 1
	case "create":
		sys.CreateNetwork()
		return 1
	default:
		return 0
	}
}

func (sys *SCPFSS) RunConsole() int {
	reader := bufio.NewReader(os.Stdin)
	var ipt string
	for {
		fmt.Print("[SCPFS@" + sys.server.dhtNode.Info.GetAddrWithPort() + "]$")
		ipt, _ = reader.ReadString('\n')
		ipt = strings.TrimSpace(ipt)
		ipt = strings.Replace(ipt, "\n", "", -1)
		ret := sys.handleCmd(ipt)
		if ret == 2 {
			sys.Quit()
			return 0
		} else if ret == 1 {
			PrintLog("Wrong Commd", ERROR)
		}
	}
}
