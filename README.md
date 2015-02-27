# sdlclient

TCP server and sdlgui client example for golang

client clock goes by server time


## requitemnts 

client 

	"github.com/veandco/go-sdl2/sdl"

	"github.com/kasworld/actionstat"
	"github.com/kasworld/go-sdlgui"
	"github.com/kasworld/go-sdlgui/analogueclock"
	"github.com/kasworld/log"
	"github.com/kasworld/netlib/gogueclient"
	"github.com/kasworld/netlib/gogueconn"
	"github.com/kasworld/runstep"

server 

	"github.com/kasworld/actionstat"
	"github.com/kasworld/idgen"
	"github.com/kasworld/log"
	"github.com/kasworld/netlib/gogueconn"
	"github.com/kasworld/netlib/gogueserver"
	"github.com/kasworld/runstep"

korean discription
http://kasw.blogspot.kr/2015/02/go-tcp-server-gui-client.html
