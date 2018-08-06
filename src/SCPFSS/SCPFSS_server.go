package SCPFSS

import (
	"chordNode"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
	//"fmt"
	"net/rpc"
)

const (
	MAX_SERVER_LIST_LEN int32 = 10
	MAX_FILE_TO_SHARE   int32 = 1024
)

type iSCPFSServer struct {
	fileShared          *fileMapper
	dhtNode             *chordNode.RingNode
	severRpc            *rpc.Server
	serverRpcService    *RpcModule
	localFileServerAddr string
	localRpcServerAddr  string
	ifInNetwork         bool
	//TODO FILE SERVER
}

type RpcModule struct {
	server *iSCPFSServer
}

func newFileMapper() *fileMapper {
	ret := new(fileMapper)
	ret.hashToFileInfo = make(map[string]SCPFSFileInfo)
	ret.filePathToHash = make(map[string]string)
	ret.idToFilePath = make([]string, 0)
	return ret
}

func newRpcModule(s *iSCPFSServer) *RpcModule {
	ret := new(RpcModule)
	ret.server = s
	return ret
}

func newSCPFSServer(lfsa, lrsa string) *iSCPFSServer {
	ret := new(iSCPFSServer)
	ret.fileShared = newFileMapper()
	ret.serverRpcService = newRpcModule(ret)
	ret.severRpc = new(rpc.Server)
	ret.localFileServerAddr = lfsa
	ret.localRpcServerAddr = lrsa
	ret.severRpc.RegisterName("SCPFSS", ret.serverRpcService)
	return ret
}

func (s *iSCPFSServer) deCodeServerList(raw string) []string {
	tmp := strings.Split(raw, ";")
	tmp = tmp[:len(tmp)-1]
	return tmp
}

func (s *iSCPFSServer) getServerList(hashedValue string) (*serverList, error) {
	rawString, ok := s.dhtNode.Get(hashedValue)
	if !ok {
		fmt.Println("Can't find the file's server list in network, please try again later")
		err := errors.New("Can't find the file's server list in network, please try again later")
		return nil, err
	} else {
		tmp := s.deCodeServerList(rawString)
		if len(rawString) <= 8 || len(tmp) == 0 {
			fmt.Println("Get invalid server list")
			err := errors.New("Get invalid server list")
			return nil, err
		} else {
			sl := new(serverList)
			sl.list = tmp
			sl.length = int32(len(sl.list))
			return sl, nil
		}
	}
}

func (s *iSCPFSServer) pingRpcServer(addr string) bool {
	var arg, ret Greet
	tconn, err := net.DialTimeout("tcp", addr, time.Duration(int64(TIME_OUT)))
	if tconn == nil || err != nil {
		return false
	}
	cl := rpc.NewClient(tconn)
	cl.Call("SCPFSS.Ping", &arg, &ret)
	cl.Close()
	if ret.Hello != "Hello" {
		return false
	}
	return true
}

func (s *iSCPFSServer) runServer() error {
	lis, err := net.Listen("tcp", s.localRpcServerAddr)
	if err != nil {
		return err
	}
	go s.severRpc.Accept(lis)
	//TODO RUN FILE SERVER
	return nil
}

func (h *RpcModule) LookUpFile(arg FileHash, ret *SCPFSFileInfo) (err error) {
	v, ok := h.server.fileShared.hashToFileInfo[arg.FileCheckSum]
	if !ok {
		err = errors.New("No such file on this server")
		return err
	} else {
		ret.LastMod = v.LastMod
		ret.Name = v.Name
		ret.Path = v.Path
		ret.Size = v.Size
		return nil
	}
}

func (h *RpcModule) Ping(arg Greet, ret *Greet) (err error) {
	ret.Hello = "Hello"
	return nil
}
