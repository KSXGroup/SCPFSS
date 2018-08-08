package SCPFSS

import (
	"chordNode"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
	//"fmt"
	"net/rpc"
)

const (
	MAX_SERVER_LIST_LEN int32 = 10
	MAX_FILE_TO_SHARE   int32 = 1024
	HEADER_LEN          int32 = 64
	SERVER_RESPONSE_LEN int32 = 64
	BUFFER_LEN          int32 = 1024
)

type iSCPFSServer struct {
	fileShared          *fileMapper
	dhtNode             *chordNode.RingNode
	severRpc            *rpc.Server
	serverRpcService    *RpcModule
	serverFileService   *iSCPFSTCPFileServer
	localFileServerAddr string
	localRpcServerAddr  string
	ifInNetwork         bool
}

type iSCPFSTCPFileServer struct {
	fileShared          *fileMapper
	fileListener        net.Listener
	localFileServerAddr string
}

type RpcModule struct {
	server      *iSCPFSServer
	rpcListener net.Listener
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
	ret.serverFileService = newSCPFSTCPServer(lfsa, ret.fileShared)
	ret.severRpc = new(rpc.Server)
	ret.localFileServerAddr = lfsa
	ret.localRpcServerAddr = lrsa
	ret.serverRpcService.rpcListener = nil
	ret.serverRpcService.server = ret
	ret.severRpc.RegisterName("SCPFSS", ret.serverRpcService)
	return ret
}

func newSCPFSTCPServer(addr string, fmp *fileMapper) *iSCPFSTCPFileServer {
	ret := new(iSCPFSTCPFileServer)
	ret.localFileServerAddr = addr
	ret.fileShared = fmp
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
			fmt.Println("rawData:" + rawString)
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

func (f *iSCPFSTCPFileServer) serveClient(client net.Conn) {
	var startPos, endPos int64
	bstartpos := make([]byte, int(HEADER_LEN))
	bendpos := make([]byte, int(HEADER_LEN))
	bufferHash := make([]byte, int(sha1Len))
	_, err := client.Read(bufferHash)
	if err != nil {
		client.Close()
		return
	}
	hv := string(bufferHash)
	info, ok := f.fileShared.hashToFileInfo[hv]
	fileToSent, err := os.Open(info.Path)
	if err != nil {
		client.Write([]byte(fillTo("Server File Error", SERVER_RESPONSE_LEN)))
		client.Close()
		return
	}
	defer fileToSent.Close()
	if !ok {
		client.Write([]byte(fillTo("File Not Found", SERVER_RESPONSE_LEN)))
		client.Close()
		return
	}
	client.Write([]byte(fillTo("Start", SERVER_RESPONSE_LEN)))
	client.Read(bstartpos)
	client.Read(bendpos)
	startPos, _ = strconv.ParseInt(strings.Replace(string(bstartpos), "/", "", -1), 10, 64)
	endPos, _ = strconv.ParseInt(strings.Replace(string(bendpos), "/", "", -1), 10, 64)
	byteToSent := endPos - startPos + 1
	if byteToSent <= 0 {
		client.Close()
		return
	}
	fileToSent.Seek(startPos, 0)
	sendBuffer := make([]byte, int(BUFFER_LEN))
	for {
		if byteToSent < int64(BUFFER_LEN) {
			tmpBuffer := make([]byte, byteToSent)
			fileToSent.Read(tmpBuffer)
			client.Write(tmpBuffer)
			break
		} else {
			_, err := fileToSent.Read(sendBuffer)
			if err == io.EOF {
				break
			}
			client.Write(sendBuffer)
		}
	}
	client.Close()
	return
}

func (f *iSCPFSTCPFileServer) runFileSever() {
	lis, err := net.Listen("tcp", f.localFileServerAddr)
	if err != nil {
		fmt.Println("Fail to start file server, please exit and try again later")
		return
	}
	f.fileListener = lis
	for {
		conn, err := f.fileListener.Accept()
		if err != nil {
			fmt.Println("Listener error, file server stopped" + err.Error())
			return
		}
		go f.serveClient(conn)
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
	s.serverRpcService.rpcListener = lis
	go s.severRpc.Accept(s.serverRpcService.rpcListener)
	go s.serverFileService.runFileSever()
	return nil
}

func (s *iSCPFSServer) stopServer() {
	if s.serverFileService.fileListener != nil {
		s.serverFileService.fileListener.Close()
	}
	if s.serverRpcService.rpcListener != nil {
		s.serverRpcService.rpcListener.Close()
	}
	s.dhtNode.Quit()
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
