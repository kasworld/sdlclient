package main

import (
	"flag"
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"time"

	"github.com/kasworld/actionstat"
	"github.com/kasworld/idgen"
	"github.com/kasworld/log"
	"github.com/kasworld/netlib/gogueconn"
	"github.com/kasworld/netlib/gogueserver"
	"github.com/kasworld/runstep"
)

type PacketToServer struct {
	Cmd int
}

type PacketToClient struct {
	Arg time.Time
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	var listenFrom = flag.String("listenFrom", ":6666", "server ip/port")
	var connCount = flag.Int("count", 1000, "connection count")
	var connThrottle = flag.Int("throttle", 10, "connection throttle")
	var rundur = flag.Int("rundur", 3600, "run sec")
	var profilefilename = flag.String("pfilename", "", "profile filename")
	flag.Parse()

	if *profilefilename != "" {
		f, err := os.Create(*profilefilename)
		if err != nil {
			log.Fatalf("profile %v", err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	server := NewServer()
	go gogueserver.TCPServer(*listenFrom, *connCount, *connThrottle, server.NewClientConn)

	server.Run(30, *rundur)
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
	clients []*ClientConn
	mutex   sync.Mutex
	stat    *actionstat.ActionStat
}

func NewServer() *Server {
	return &Server{
		clients: make([]*ClientConn, 0),
		stat:    actionstat.NewActionStat(),
	}
}
func (s *Server) AddClient(c *ClientConn) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.clients = append(s.clients, c)
}

func (s *Server) DelClient(c *ClientConn) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	for i, v := range s.clients {
		if c.id == v.id {
			s.clients = append(s.clients[:i], s.clients[i+1:]...)
			return
		}
	}
	log.Error("client not found", c)
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
	id    int64
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
