package SCPFSS

import (
	"bufio"
	"chordNode"
	"errors"
	"fmt"
	"io"
	"math"
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
	startShareInfo  string = "share <YourFilePath>"
	stopShareInfo   string = "stopShare <YourFilePath>"
	findInfo        string = "find <SCPFSS LINK>"
	getInfo         string = "sget <SCPFSS LINK>"
	joinInfo        string = "join <IP:Port>"
	sgetInfo        string = "sget <SCPFSS LINK>"
	sgetmInfo       string = "sgetm <Thread Number> <SCPFSS LINK>"
	linkPrefix      string = "SCPFSP:?h=SHA1:"
	defaultDhtPort  int32  = 1919
	defaultFilePort int32  = 2020
	fileChunk       int32  = 4096
	linkLen         int32  = 55
	sha1Len         int32  = 40
	minFileChunk    int64  = 1024 * 1024
	TIME_OUT        int64  = 1e9
	INIT            uint8  = 0
	GETTING         uint8  = 1
	FIN             uint8  = 2
	ERROR           uint8  = 3
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

type fileGeter struct {
	sourceAddr   string
	sourceHashId string
	savePath     string
	start        int64
	end          int64
	byteRecv     int64
	byteToGet    int64
	taskId       int32
	status       uint8
	headerBuffer []byte
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
	ret.server.dhtNode = dht
	return ret

}

func newFileGetter(addr, hashid, svp string, st, ed int64, id int32) *fileGeter {
	ret := new(fileGeter)
	ret.sourceAddr = addr
	ret.sourceHashId = hashid
	ret.savePath = svp
	ret.start = st
	ret.end = ed
	ret.taskId = id
	ret.headerBuffer = make([]byte, int(HEADER_LEN))
	ret.status = INIT
	return ret
}

func (g *fileGeter) startGet(wg *sync.WaitGroup) error {
	defer wg.Done()
	g.status = GETTING
	if g.start < 0 || g.end < 0 || g.savePath == "" || g.sourceAddr == "" || g.sourceHashId == "" || g.taskId < 0 {
		fmt.Printf("Invalid parameter on task #%d", g.taskId)
		g.status = ERROR
		return errors.New("Invalid parameter(s)")
	}
	g.byteToGet = g.end - g.start + 1
	if g.byteToGet <= 0 {
		g.status = ERROR
		return nil
	}
	g.byteRecv = 0
	iconn, ierr := net.DialTimeout("tcp", g.sourceAddr, time.Duration(TIME_OUT))
	if ierr != nil {
		fmt.Printf("task#%d: %s\n", g.taskId, "Dial remote node error:"+ierr.Error())
		if iconn != nil {
			iconn.Close()
		}
		g.status = ERROR
		return ierr
	}
	newFile, ferr := os.Create(g.savePath + ".tmp" + strconv.Itoa(int(g.taskId)))
	if ferr != nil {
		fmt.Printf("task#%d: %s", g.taskId, "Create file fail:"+ferr.Error())
		return ferr
	}
	defer newFile.Close()
	iconn.Write([]byte(g.sourceHashId))
	_, ioerr := iconn.Read(g.headerBuffer)
	if ioerr != nil {
		fmt.Printf("task#%d: %s", g.taskId, "Transfer error, please retry later")
		if iconn != nil {
			iconn.Close()
		}
		g.status = ERROR
		return errors.New("Transfer error, please retry later")
	}
	response := strings.Replace(string(g.headerBuffer), "/", "", -1)
	response = strings.TrimSpace(response)
	if response != "Start" {
		fmt.Println("Server give incorrect response, download terminated")
		ioerr := errors.New("Server give incorrect response, download terminated")
		iconn.Close()
		g.status = ERROR
		return ioerr
	}
	iconn.Write([]byte(fillTo(strconv.FormatInt(g.start, 10), HEADER_LEN)))
	iconn.Write([]byte(fillTo(strconv.FormatInt(g.end, 10), HEADER_LEN)))
	for {
		if g.byteToGet-g.byteRecv < int64(BUFFER_LEN) {
			io.CopyN(newFile, iconn, int64(g.byteToGet-g.byteRecv))
			g.byteRecv += g.byteToGet - g.byteRecv
			break
		}
		io.CopyN(newFile, iconn, int64(BUFFER_LEN))
		g.byteRecv += int64(BUFFER_LEN)
	}
	iconn.Close()
	g.status = FIN
	return nil
}

func (sys *SCPFSS) GetFileMultiThread(link string, savePath string, threadCount int32) error {
	fmt.Println("Start getting files")
	//var recvCnt int64 = 0
	var wg sync.WaitGroup
	var cminChunk int64 = 0
	hashid := strings.Replace(link, linkPrefix, "", -1)
	ok, fileInfo, err := sys.LookUpFile(link)
	if err != nil || !ok {
		fmt.Println("Fail to look up, please check your link")
		return errors.New("Fail to look up fail, please check your link")
	}
	slist, serr := sys.server.getServerList(hashid)
	if serr != nil {
		return errors.New("Error when get serverlist")
	}
	if float64(fileInfo.Size/int64(threadCount)) < float64(minFileChunk) {
		threadCount = int32(math.Floor(float64(fileInfo.Size)/float64(minFileChunk) + 0.5))
		cminChunk = int64(minFileChunk)
	} else {
		cminChunk = int64(math.Floor(float64(fileInfo.Size)/float64(threadCount) + 0.5))
	}
	var getterGroup = make([]*fileGeter, threadCount)
	var st int64 = 0
	for i := int32(1); i <= threadCount-1; i += 1 {
		getterGroup[i-1] = newFileGetter(slist.list[(i-1)%int32(len(slist.list))], hashid, savePath+fileInfo.Name, st, st+cminChunk, i-1)
		fmt.Printf("task#%d get %d to %d\n", i-1, st, st+cminChunk)
		st += cminChunk + 1
	}
	if threadCount == 0 {
		threadCount += 1
	}
	getterGroup[threadCount-1] = newFileGetter(slist.list[(threadCount-1)%int32(len(slist.list))], hashid, savePath+fileInfo.Name, st, fileInfo.Size-1, threadCount-1)
	fmt.Printf("task#%d get %d to %d\n", threadCount-1, st+1, fileInfo.Size)
	for i := 0; i < int(threadCount); i += 1 {
		wg.Add(1)
		go getterGroup[i].startGet(&wg)
	}
	wg.Wait()
	mergeBuffer := make([]byte, fileChunk)
	fileFlag := true
	fmt.Println("Get finished, merge file together and check")
	fileMergedTo, fmerr := os.OpenFile(savePath+fileInfo.Name+strconv.Itoa(0), os.O_WRONLY|os.O_APPEND, 0666)
	if fmerr != nil {
		fmt.Println(fmerr.Error())
		for i := 0; i <= int(threadCount-1); i += 1 {
			os.Remove(savePath + fileInfo.Name + strconv.Itoa(i))
		}
	}
	for i := 1; i <= int(threadCount-1); i += 1 {
		ftmp, ferr := os.Open(savePath + fileInfo.Name + strconv.Itoa(i))
		if ferr != nil {
			fmt.Println(ferr.Error())
			fileFlag = false
			break
		}
		for {
			_, err := ftmp.Read(mergeBuffer)
			if err == io.EOF {
				break
			}
			fileMergedTo.Write(mergeBuffer)
		}
		ftmp.Close()
		os.Remove(savePath + fileInfo.Name + strconv.Itoa(i))
	}
	fileMergedTo.Close()
	if !fileFlag {
		for i := 0; i <= int(threadCount-1); i += 1 {
			os.Remove(savePath + fileInfo.Name + strconv.Itoa(i))
		}
		return errors.New("Get error when merge files")
	} else {
		os.Rename(savePath+fileInfo.Name+strconv.Itoa(0), savePath+fileInfo.Name)
		fmt.Println("Start check file")
		v, herr := sha1HashFile(savePath + fileInfo.Name)
		if herr != nil {
			fmt.Println("Hash error: " + herr.Error())
			return herr
		}
		if v == hashid {
			fmt.Println("Success")
			return nil
		} else {
			fmt.Println("Check fail, please retry")
			return errors.New("Check fail, please retry")
		}
	}
}

func (sys *SCPFSS) GetFile(link string, savePath string) error {
	fmt.Println("Start getting files")
	//var recvCnt int64 = 0
	var startPos, endPos int64
	var wg sync.WaitGroup
	hashid := strings.Replace(link, linkPrefix, "", -1)
	ok, fileInfo, err := sys.LookUpFile(link)
	if err != nil || !ok {
		fmt.Println("Fail to look up, please check your link")
		return errors.New("Fail to look up fail, please check your link")
	}
	startPos = 0
	endPos = fileInfo.Size - 1
	slist, serr := sys.server.getServerList(hashid)
	if serr != nil {
		return errors.New("Error when get serverlist")
	}
	remoteAddr := slist.list[0]
	getter := newFileGetter(remoteAddr, hashid, savePath+fileInfo.Name, startPos, endPos, 0)
	fmt.Println("Getting file")
	wg.Add(1)
	go getter.startGet(&wg)
	wg.Wait()
	fmt.Println("Finished")
	if getter.byteRecv != getter.byteToGet {
		fmt.Println("Error occurred when getting file, please retry later")
		return errors.New("Error occurred when getting file, please retry later")
	}
	os.Rename(savePath+fileInfo.Name+".tmp"+strconv.Itoa(int(getter.taskId)), savePath+fileInfo.Name)
	fmt.Println("Recheck file SHA1")
	hashOfNewFile, err := sha1HashFile(savePath + fileInfo.Name)
	if hashOfNewFile == hashid {
		fmt.Println("Success")
		return nil
	} else {
		fmt.Println("Check fail, please retry")
		return errors.New("Check fail, please retry")
	}
}

func (sys *SCPFSS) Quit() {
	if sys.server.ifInNetwork == false || sys.server.dhtNode.InRing == false {
		return
	} else {
		for k, _ := range sys.server.fileShared.filePathToHash {
			sys.StopShare(k)
		}
		sys.server.stopServer()
		return
	}
}

func (sys *SCPFSS) Share(filePath string) (string, error) {
	if sys.server.ifInNetwork == false || sys.server.dhtNode.InRing == false {
		err := errors.New("Not in the SCPFS Network")
		fmt.Println("Not in SCPFS network")
		return "", err
	}
	var v1 string
	var v2 SCPFSFileInfo
	var ok1, ok2 bool
	v1, ok1 = sys.server.fileShared.filePathToHash[filePath]
	if ok1 {
		rerr := errors.New("File is still sharing")
		fmt.Println("This file has been shared before, please note down the link below:")
		fmt.Println(linkPrefix + v1)
		return linkPrefix + v1, rerr
	}
	fileCheckSum, rerr := sha1HashFile(filePath)
	if rerr != nil {
		fmt.Println("Hash fail, invalid file")
		return "", rerr
	}
	v2, ok2 = sys.server.fileShared.hashToFileInfo[fileCheckSum]
	if ok2 {
		rerr := errors.New("File is still sharing")
		fmt.Println("This file has been shared before, please note down the link below:")
		fmt.Println(linkPrefix + fileCheckSum)
		return linkPrefix + fileCheckSum, rerr
	}
	file, _ := os.Open(filePath)
	defer file.Close()
	info, fserr := file.Stat()
	if fserr != nil {
		return "", fserr
	}
	if info.IsDir() {
		err := errors.New("Invalid file, do not use directory to cheat me")
		fmt.Println("Invalid file, do not use directory to cheat me")
		return "", err
	}
	v2.LastMod = info.ModTime()
	v2.Size = info.Size()
	v2.Name = info.Name()
	v2.Path = filePath
	sys.server.fileShared.idToFilePath = append(sys.server.fileShared.idToFilePath, filePath)
	sys.server.fileShared.filePathToHash[filePath] = fileCheckSum
	sys.server.fileShared.hashToFileInfo[fileCheckSum] = v2
	ret := sys.server.dhtNode.AppendToData(fileCheckSum, sys.localFileServerAddr+";")
	if ret == 2 {
		err := errors.New("This file has been shared before")
		fmt.Println("This file has been shared before, please note down the link below:")
		fmt.Println(linkPrefix + fileCheckSum)
		return linkPrefix + fileCheckSum, err
	} else if ret == 1 {
		fmt.Println("File shared successfully, please note down the link below and share with others:")
		fmt.Println(linkPrefix + fileCheckSum)
		return linkPrefix + fileCheckSum, nil
	} else {
		return "", nil
	}
}

func (sys *SCPFSS) StopShare(filePath string) (bool, error) {
	if sys.server.ifInNetwork == false || sys.server.dhtNode.InRing == false {
		err := errors.New("Not in the SCPFS Network")
		fmt.Println("Not in SCPFS network")
		return false, err
	}
	var toDelHash, toDelPath string
	var toDelId int
	v1, ok1 := sys.server.fileShared.filePathToHash[filePath]
	if !ok1 {
		err := errors.New("File has not been shared before")
		fmt.Println("File has not been shared before")
		return false, err
	}
	ret := sys.server.dhtNode.RemoveFromData(v1, sys.localFileServerAddr+";")
	if ret == 0 {
		err := errors.New("File remove failed for SCPFS reasons, please try again later")
		fmt.Println("File remove failed for SCPFS reasons, please try again later")
		return false, err
	} else if ret == 3 {
		err := errors.New("File not found in SCPFS network")
		fmt.Println("File not found in SCPFS network")
		return false, err
	}
	for k, v := range sys.server.fileShared.idToFilePath {
		if v == filePath {
			fmt.Println("File found")
			toDelPath = v
			toDelId = k
			break
		}
	}
	toDelHash = sys.server.fileShared.filePathToHash[toDelPath]
	delete(sys.server.fileShared.filePathToHash, toDelPath)
	delete(sys.server.fileShared.hashToFileInfo, toDelHash)
	sys.server.fileShared.idToFilePath = append(sys.server.fileShared.idToFilePath[:toDelId], sys.server.fileShared.idToFilePath[toDelId:]...)
	fmt.Println("File stop sharing")
	return true, nil
}

func (sys *SCPFSS) JoinNetwork(addr string) (bool, error) {
	var hasFileServer bool = false
	if len(addr) < 7 {
		err := errors.New("Invalid Addr")
		return false, err
	}
	if !sys.server.dhtNode.Join(addr) {
		err := errors.New("Fail to join DHT network")
		fmt.Println("Fail to join DHT network")
		return false, err
	} else {
		fmt.Println("Join DHT network successfully")
	}
	for i := 1; i <= 100; i += 1 {
		ip := strings.Split(addr, ":")[0] + ":" + strconv.Itoa(int(defaultFilePort)+i)
		if sys.server.pingRpcServer(ip) {
			fmt.Println("Detected remote file server")
			hasFileServer = true
			break
		}
	}
	if hasFileServer {
		fmt.Println("Join SCPFS Network successfully")
		sys.server.ifInNetwork = true
		return true, nil
	} else {
		ferr := errors.New("Remote address has no file server")
		return false, ferr
	}
}

func (sys *SCPFSS) CreateNetwork() (bool, error) {
	fmt.Println("Create new SCPFS Network")
	sys.server.dhtNode.Create()
	sys.server.ifInNetwork = true
	return true, nil
}

func (sys *SCPFSS) LookUpFile(link string) (bool, *SCPFSFileInfo, error) {
	if sys.server.ifInNetwork == false || sys.server.dhtNode.InRing == false {
		fmt.Println("Not in the SCPFS Network")
		err := errors.New("Not in the SCPFS Network")
		return false, nil, err
	}
	if len(link) != int(linkLen) {
		fmt.Println("Invalid SCPFS Link")
		err := errors.New("Invalid SCPFS Link")
		return false, nil, err
	}
	var cl *rpc.Client
	var ret *SCPFSFileInfo
	var arg FileHash
	var goodFlag bool = false
	hashid := strings.Replace(link, linkPrefix, "", -1)
	ret = new(SCPFSFileInfo)
	sl, err := sys.server.getServerList(hashid)
	if err != nil {
		return false, nil, err
	}
	fmt.Println("Get avaliable node list:")
	for _, v := range sl.list {
		if v != "" {
			fmt.Println(v)
		}
	}
	if len(sl.list) <= 0 {
		lerr := errors.New("Invalid server list")
		fmt.Println("Invalid server list")
		return false, nil, lerr
	}
	arg.FileCheckSum = hashid
	for _, item := range sl.list {
		fmt.Println("Try " + item)
		ts := strings.Split(item, ":")
		tip := ts[0]
		tport, perr := strconv.Atoi(ts[1])
		if perr != nil {
			fmt.Println(item + " is invalid")
			continue
		}
		tports := strconv.Itoa(tport + 1)
		taddr := tip + ":" + tports
		tconn, cerr := net.DialTimeout("tcp", taddr, time.Duration(TIME_OUT))
		if cerr != nil || tconn == nil {
			tconn = nil
			fmt.Println(taddr + " Fail")
		} else {
			cl = rpc.NewClient(tconn)
			rpcErr := cl.Call("SCPFSS.LookUpFile", &arg, ret)
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
		return false, nil, ferr
	}
	return true, ret, nil
}

func (sys *SCPFSS) ListFileShared() {
	if sys.server.ifInNetwork == false || sys.server.dhtNode.InRing == false {
		fmt.Println("[Not in the SCPFSS]")
		return
	}
	if len(sys.server.fileShared.hashToFileInfo) == 0 {
		fmt.Println("[No file shared]")
		return
	}
	for k, v := range sys.server.fileShared.idToFilePath {
		if v == "" {
			break
		}
		fmt.Printf("#%d ", k+1)
		chash := sys.server.fileShared.filePathToHash[v]
		info, ok := sys.server.fileShared.hashToFileInfo[chash]
		if ok {
			info.Print()
		} else {
			fmt.Println("[ERROR]")
		}
	}
}

func (sys *SCPFSS) handleCmd(cmd string) int {
	if len(cmd) <= 0 {
		return 1
	}
	var cnt int
	var err error
	splitedCmd := strings.Fields(cmd)
	switch splitedCmd[0] {
	case "exit":
		sys.Quit()
		return 2
	case "share":
		if len(splitedCmd) == 2 {
			sys.Share(splitedCmd[1])
		} else {
			fmt.Println(startShareInfo)
		}
		return 1
	case "stopshare":
		if len(splitedCmd) == 2 {
			sys.StopShare(splitedCmd[1])
		} else {
			fmt.Println(stopShareInfo)
		}
		return 1
	case "join":
		if len(splitedCmd) == 2 {
			sys.JoinNetwork(splitedCmd[1])
		} else {
			fmt.Println(joinInfo)
		}
		return 1
	case "find":
		if len(splitedCmd) == 2 {
			sys.LookUpFile(splitedCmd[1])
		} else {
			fmt.Println(findInfo)
		}
		return 1
	case "create":
		sys.CreateNetwork()
		return 1
	case "ls":
		sys.ListFileShared()
		return 1
	case "sget":
		if len(splitedCmd) == 2 {
			sys.GetFile(splitedCmd[1], "")
		} else {
			fmt.Println(sgetInfo)
		}
		return 1
	case "sgetm":
		cnt, err = strconv.Atoi(splitedCmd[1])
		if len(splitedCmd) == 3 && err == nil {
			sys.GetFileMultiThread(splitedCmd[2], "", int32(cnt))
			return 1
		} else {
			fmt.Println(sgetmInfo)
			return 1
		}
	case "info":
		fmt.Println("DHT: " + sys.localDhtAddr)
		fmt.Println("FileServer: " + sys.localFileServerAddr)
		fmt.Println("FileRPC: " + sys.localRpcServerAddr)
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
		fmt.Print("[SCPFS@" + sys.server.dhtNode.Info.GetAddrWithPort() + "]$ ")
		ipt, _ = reader.ReadString('\n')
		ipt = strings.TrimSpace(ipt)
		ipt = strings.Replace(ipt, "\n", "", -1)
		ret := sys.handleCmd(ipt)
		if ret == 2 {
			sys.Quit()
			return 0
		} else if ret == 0 {
			fmt.Println("Wrong Commd")
		}
	}
}
