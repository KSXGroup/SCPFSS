package SCPFSS

import (
	"chordNode"
	"errors"
	//"fmt"
)

const (
	MAX_SERVER_LIST_LEN int32 = 10
	MAX_FILE_TO_SHARE   int32 = 1024
)

type fileMapper struct {
	hashToFilePath map[string]string
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

func newFileMapper() *fileMapper {
	ret := new(fileMapper)
	ret.hashToFilePath = make(map[string]string)
	ret.filePathToHash = make(map[string]string)
	ret.idToFilePath = make([]string, int(MAX_FILE_TO_SHARE))
	return ret
}

func newSCPFSServer() *iSCPFSServer {
	ret := new(iSCPFSServer)
	ret.fileShared = new(fileMapper)
	return ret
}

func (s *iSCPFSServer) getServerList(hashedValue string) (*serverList, error) {
	rawString, ok := s.dhtNode.Get()
	if !ok {
		err := errors.New("Can find the file's server list in network, please try again later")
		return nil, err
	} else {
		if len(rawString) <= 8 {

		}
	}
}
