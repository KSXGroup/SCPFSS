package SCPFSS

import (
	"chordNode"
	"errors"
	"strings"
	"time"
	//"fmt"
)

const (
	MAX_SERVER_LIST_LEN int32 = 10
	MAX_FILE_TO_SHARE   int32 = 1024
)

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

type iSCPFSServer struct {
	fileShared  *fileMapper
	dhtNode     *chordNode.RingNode
	ifInNetwork bool
	//TODO FILE SERVER
}

type RPCModule struct {
}

func newFileMapper() *fileMapper {
	ret := new(fileMapper)
	ret.hashToFileInfo = make(map[string]SCPFSFileInfo)
	ret.filePathToHash = make(map[string]string)
	ret.idToFilePath = make([]string, int(MAX_FILE_TO_SHARE))
	return ret
}

func newSCPFSServer() *iSCPFSServer {
	ret := new(iSCPFSServer)
	ret.fileShared = new(fileMapper)
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
		err := errors.New("Can find the file's server list in network, please try again later")
		return nil, err
	} else {
		tmp := s.deCodeServerList(rawString)
		if len(rawString) <= 8 || len(tmp) == 0 {
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
func 