// Copyright 2015 SeukWon Kang (kasworld@gmail.com)
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//    http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"flag"
	"runtime"
	"sync"
	"time"

	"github.com/kasworld/actionstat"
	"github.com/kasworld/idgen"
	"github.com/kasworld/log"
	"github.com/kasworld/netlib/gogueconn"
	"github.com/kasworld/netlib/gogueserver"
	"github.com/kasworld/runstep"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	var listenFrom = flag.String("listenFrom", ":6666", "server ip/port")
	var connCount = flag.Int("count", 1000, "connection count")
	var connThrottle = flag.Int("throttle", 10, "connection throttle")
	var rundur = flag.Int("rundur", 3600, "run sec")
	flag.Parse()

	server := NewServer()
	go gogueserver.TCPServer(*listenFrom, *connCount, *connThrottle, server.NewClientConn)
	server.Run(30, *rundur)
}

type PacketToServer struct {
	Cmd int
}

type PacketToClient struct {
	Arg time.Time
}

func (s *Server) Run(fps int, dur int) {
	timerFrame := time.Tick(time.Duration(1000000000/fps) * time.Nanosecond)
	timerInfoCh := time.Tick(time.Duration(1000) * time.Millisecond)
	timerQuit := time.Tick(time.Duration(dur) * time.Second)
	for {
		select {
		case <-timerFrame:
			s.recvClient()
			s.SendClient()
		case <-timerInfoCh:
			log.Info("%v", s.stat)
			s.stat.UpdateLap()
		case <-timerQuit:
			break
		}
	}
}

type Server struct {
	clients map[idgen.IDInt]*ClientConn
	mutex   sync.Mutex
	stat    *actionstat.ActionStat
}

func NewServer() *Server {
	return &Server{
		clients: make(map[idgen.IDInt]*ClientConn, 0),
		stat:    actionstat.New(),
	}
}
func (s *Server) AddClient(c *ClientConn) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.clients[c.id] != nil {
		log.Error("client exist %v", c)
		return
	}
	s.clients[c.id] = c
}

func (s *Server) DelClient(c *ClientConn) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.clients[c.id] == nil {
		log.Error("client not exist %v", c)
		return
	}
	delete(s.clients, c.id)
}

func (s *Server) recvClient() {
	for _, v := range s.clients {
		<-v.ResultCh()
		s.stat.Inc()
	}
}

func (s *Server) SendClient() {
	for _, v := range s.clients {
		v.StartStepCh() <- &PacketToClient{time.Now()}
	}
}

func (s *Server) NewClientConn(gconn *gogueconn.GogueConn, clientQueue <-chan bool) {
	defer gconn.Close()
	defer func() { <-clientQueue }()
	// log.Info("client connected")

	c := NewClientConn(gconn)
	s.AddClient(c)
	defer s.DelClient(c)
	c.Run()
}

type ClientConn struct {
	*runstep.RunStep
	id    idgen.IDInt
	gconn *gogueconn.GogueConn
}

func NewClientConn(gconn *gogueconn.GogueConn) *ClientConn {
	rtn := ClientConn{
		gconn: gconn,
		id:    <-idgen.GenCh(),
	}
	rtn.RunStep = runstep.New(1)
	return &rtn
}

func (cc *ClientConn) Run() {
loop:
	for {
		var rdata PacketToServer
		err := cc.gconn.Recv(&rdata)
		if err != nil {
			if err.Error() != "EOF" {
				log.Error("%v server recv %v\n", cc, err)
			}
			break loop
		}
		cc.SendStepResult(rdata)
		// wait server action
		sdata := cc.RecvStepArg().(*PacketToClient)
		err = cc.gconn.Send(sdata)
		if err != nil {
			if err.Error() != "EOF" {
				log.Error("%v server send %v\n", cc, err)
			}
			break loop
		}
	}
}
