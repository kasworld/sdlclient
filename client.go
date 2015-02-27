package main

import (
	"flag"
	"runtime"
	"time"

	"github.com/veandco/go-sdl2/sdl"

	"github.com/kasworld/actionstat"
	"github.com/kasworld/go-sdlgui"
	"github.com/kasworld/go-sdlgui/analogueclock"
	"github.com/kasworld/log"
	"github.com/kasworld/netlib/gogueclient"
	"github.com/kasworld/netlib/gogueconn"
	"github.com/kasworld/runstep"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	var connectTo = flag.String("connectTo", "localhost:6666", "server ip/port")
	flag.Parse()

	app := NewApp(*connectTo)
	if app == nil {
		return
	}
	app.Run()
}

type PacketToServer struct {
	Cmd int
}

type PacketToClient struct {
	Arg time.Time
}

type serverConn struct {
	*runstep.RunStep
	Gconn *gogueconn.GogueConn
}

func NewServerConn(connectTo string) *serverConn {
	rtn := &serverConn{}
	rtn.Gconn = gogueclient.NewClientGogueConn(connectTo)
	if rtn.Gconn == nil {
		return nil
	}
	rtn.RunStep = runstep.New(0)
	return rtn
}

func (sc *serverConn) Step(d interface{}) interface{} {
	switch d.(type) {
	case PacketToServer:
		sd := d.(PacketToServer)
		err := sc.Gconn.Send(sd)
		if err != nil {
			log.Error("%v send fail %v", sd, err)
			return nil
		}

		var rdata PacketToClient
		err = sc.Gconn.Recv(&rdata)
		if err != nil {
			log.Error("%v recv fail %v", sd, err)
			return nil
		}
		return &rdata
	default:
		log.Error("unknown packet %v", d)
		return nil
	}
}

type App struct {
	Quit     bool
	SdlCh    chan interface{}
	Keys     sdlgui.KeyState
	Win      *sdlgui.Window
	Controls sdlgui.ControlIList

	gconn *serverConn

	cl   *analogueclock.Clock
	Stat *actionstat.ActionStat
}

func NewApp(connectTo string) *App {
	app := App{
		SdlCh: make(chan interface{}, 1),
		Keys:  make(map[sdl.Scancode]bool),
		Win:   sdlgui.NewWindow("SDL GUI Clock Example", 512, 512, true),

		Stat:  actionstat.NewActionStat(),
		gconn: NewServerConn(connectTo),
	}
	if app.gconn == nil {
		return nil
	}
	app.addControls()
	app.Win.UpdateAll()
	return &app
}

func (app *App) AddControl(c sdlgui.ControlI) {
	app.Controls = append(app.Controls, c)
	app.Win.AddControl(c)
}

// change as app's need

func (g *App) addControls() {
	g.cl = analogueclock.New(0, 0, 0, 512, 512)
	g.AddControl(g.cl)
}

func (app *App) Run() {
	go app.gconn.Run(app.gconn.Step)
	defer app.gconn.Quit()

	// need to co-exist sdl lib
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// start sdl event loop
	sdlgui.SDLEvent2Ch(app.SdlCh)

	timerInfoCh := time.Tick(time.Duration(1000) * time.Millisecond)
	timerDrawCh := time.Tick(time.Duration(1000/30) * time.Millisecond)
	app.gconn.StartStepCh() <- PacketToServer{0}

	for !app.Quit {
		select {
		case data := <-app.SdlCh:
			if app.Win.ProcessSDLMouseEvent(data) ||
				app.Keys.ProcessSDLKeyEvent(data) {
				app.Quit = true
			}
			app.Stat.Inc()

		case rdata := <-app.gconn.ResultCh():
			if rdata == nil {
				app.Quit = true
				break
			}
			rpacket := rdata.(*PacketToClient)
			app.cl.SetTime(rpacket.Arg)

			app.gconn.StartStepCh() <- PacketToServer{0}

		case <-timerDrawCh:
			for _, v := range app.Controls {
				v.DrawSurface()
			}
			app.Win.Update()

		case <-timerInfoCh:
			log.Info("stat %v", app.Stat)
			app.Stat.UpdateLap()
		}
	}
}
