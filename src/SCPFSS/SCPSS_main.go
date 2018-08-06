package SCPFSS

import (
	"bufio"
	"chordNode"
	"errors"
	"fmt"
	"net"
	"net/rpc"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	welcomeInfo     string = "Welcome to use Stupid Chord Peer to peer File Sharing System"
	helpInfo        string = "There is no help info, you are on your own, you can choose to uninstall this stupid software."
	startShareInfo  string = "Share <YourFilePath>"
	stopShareInfo   string = "StopShare <YourFilePath>"
	findInfo        string = "Find <SCPFSS LINK>"
	joinInfo        string = "Join <IP:Port>"
	linkPrefix      string = "SCPFSP:?h=SHA1:"
	defaultDhtPort  int32  = 1919
	defaultFilePort int32  = 2020
	fileChunk       int32  = 4096
	TIME_OUT        int64  = 1e9
)

type SCPFSS struct {
	server              *iSCPFSServer
	wg                  *sync.WaitGroup
	localDhtAddr        string
	localFileServerAddr string
	localRpcServerAddr  string
	maxCoroutine        int32
	Timeout             int32
}

func NewSCPFSS() *SCPFSS {
	ret := new(SCPFSS)
	port := findFreePort(defaultDhtPort)
	dht := chordNode.NewNode(port)
	ret.localDhtAddr = getIp() + ":" + strconv.Itoa(int(port))
	port = findFreePort(defaultFilePort)
	ret.localFileServerAddr = getIp() + ":" + strconv.Itoa(int(port))
	ret.localRpcServerAddr = getIp() + ":" + strconv.Itoa(int(port+1))
	ret.server = newSCPFSServer(ret.localFileServerAddr, ret.localRpcServerAddr)
	ret.maxCoroutine = 10
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
	var v1 string
	var v2 SCPFSFileInfo
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
	v2, ok2 = sys.server.fileShared.hashToFileInfo[fileCheckSum]
	if ok2 {
		rerr := errors.New("File is still sharing")
		return linkPrefix + v2.Path, rerr
	}
	sys.server.fileShared.idToFilePath = append(sys.server.fileShared.idToFilePath, fileCheckSum)
	file, _ := os.Open(filePath)
	defer file.Close()
	info, fserr := file.Stat()
	if fserr != nil {
		return "", fserr
	}
	if info.IsDir() {
		err := errors.New("Invalid file, do not use directory to cheat me")
		return "", err
	}
	v2.LastMod = info.ModTime()
	v2.Size = info.Size()
	v2.Name = info.Name()
	v2.Path = filePath
	sys.server.fileShared.filePathToHash[filePath] = fileCheckSum
	sys.server.fileShared.hashToFileInfo[fileCheckSum] = v2
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
	var toDelHash, toDelPath string
	var toDelId int
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
	for k, v := range sys.server.fileShared.idToFilePath {
		if v == filePath {
			toDelPath = v
			toDelId = k
			break
		}
	}
	toDelHash = sys.server.fileShared.filePathToHash[toDelPath]
	delete(sys.server.fileShared.filePathToHash, toDelPath)
	delete(sys.server.fileShared.hashToFileInfo, toDelHash)
	sys.server.fileShared.idToFilePath = append(sys.server.fileShared.idToFilePath[:toDelId], sys.server.fileShared.idToFilePath[toDelId:]...)
	return true, nil
}

func (sys *SCPFSS) JoinNetwork(addr string) (bool, error) {
	var hasFileServer bool = false
	if len(add) < 7 {
		err := errors.New("Invalid Addr")
		return false, err
	}
	if !sys.server.dhtNode.Join(addr) {
		err := errors.New("Fail to join DHT network")
		return false, err
	}
	for i := 1; i <= 100; i += 1 {
		ip = strings.Split(addr, ":")[0] + strconv.Itoa(int(defaultFilePort)+i)
		if sys.server.pingRpcServer(ip) {
			hasFileServer = true
			break
		}
	}
	if hasFileServer {
		sys.server.ifInNetwork = true
		return true, nil
	} else {
		ferr := errors.New("Remote address has no file server")
		return false, ferr
	}
}

func (sys *SCPFSS) CreateNetwork() (bool, error) {
	sys.server.dhtNode.Create()
	sys.server.ifInNetwork = true
	return true, nil
}

func (sys *SCPFSS) LookUpFile(link string) (bool, error) {
	if sys.server.ifInNetwork == false || sys.server.dhtNode.InRing == false {
		err := errors.New("Not in the SCPFS Network")
		return false, err
	}
	var cl *rpc.Client
	var ret SCPFSFileInfo
	var arg FileHash
	var goodFlag bool = false
	hashid := strings.Replace(link, linkPrefix, "", -1)
	sl, err := sys.server.getServerList(hashid)
	if err != nil {
		return false, err
	}
	fmt.Println("Get avaliable node list:")
	for _, v := range sl.list {
		if v != "" {
			fmt.Println(v)
		}
	}
	if len(sl.list) <= 0 {
		lerr := errors.New("Invalid server list")
		return false, lerr
	}
	arg.FileCheckSum = hashid
	for _, item := range sl.list {
		fmt.Println("Try " + item)
		tconn, cerr := net.DialTimeout("tcp", item, time.Duration(TIME_OUT))
		if cerr != nil || tconn == nil {
			tconn.Close()
			tconn = nil
		} else {
			cl = rpc.NewClient(tconn)
			rpcErr := cl.Call("SCPFSS.LookUpFile", &arg, &ret)
			cl.Close()
			if rpcErr == nil {
				ret.Print()
				goodFlag = true
				break
			}
		}
	}
	if !goodFlag {
		ferr := errors.New("All nodes are fail, file lost")
		return false, ferr
	}
	return true, nil
}

func (sys *SCPFSS) ListFileShared() {
	if sys.server.ifInNetwork == false || sys.server.dhtNode.InRing == false {
		return
	}
	if len(sys.server.fileShared.hashToFileInfo) == 0 {
		fmt.Print("[No file shared]")
	}
	for _, v := range sys.server.fileShared.hashToFileInfo {
		v.Print()
	}
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
	case "ls":
		sys.ListFileShared()
		return 1
	default:
		return 0
	}
}

func (sys *SCPFSS) RunConsole() int {
	sys.server.runServer()
	reader := bufio.NewReader(os.Stdin)
	var ipt string
	fmt.Println(welcomeInfo)
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
