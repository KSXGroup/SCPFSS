# Stupid Chord Peer to peer File Share System#

## 1.Introduction

Stupid Chord Peer to peer File Share System or "SCPFSS" is a peer to peer file sharing system based on Distributed Hash Table implemented with Chord, A scalable peer-to-peer look up protocol for Internet Applications.The Chord system which SCPFSS based on can be found here:https://github.com/KSXGroup/stupidDHT

SCPFSS post the file hash as key and your IP as value into DHT network, and anyone in the network can get any part of the file you shared on any computer in the network anytime as long as you keep sharing them, however you can stop share your file anytime. 

You can get files from nodes that share the same file, with multi-thread technique, the downloading speed can be very fast.

## 2.User Manual

### create

Command to create a new SCPFS Network, you can invite anyone to join, and a center server is unnecessary.

### join \< IP Address > : \<Port> 

Command to join a SCPFS Network, you can join via any address in the network

### share \<filePath>

start share a file to network

### stopshare \<filePath>

stop share a file to network

### sget \<SCPFS LINK>

get a file from a single node with SCPFS LINK

### sgetm \<thread number> \<SCPFS LINK>

get a file with \<thread number> of thread getting at the same time

### ls

list all file this node shared in the network

### info

show the information about this server 

### help

show help info