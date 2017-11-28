package socket

import (
	"net"
	"github.com/FlowerWrong/netstack/waiter"
	"github.com/FlowerWrong/netstack/tcpip"
	"log"
	"io"
	"golang.org/x/net/proxy"
	"fmt"
)

type Tunnel struct {
	wq         *waiter.Queue
	ep         tcpip.Endpoint
	socks5Conn net.Conn
}

func NewTunnel(wq *waiter.Queue, ep tcpip.Endpoint) *Tunnel {
	// connect to socks5
	network := "tcp"
	socks5Addr := "127.0.0.1:1090"

	dialer, socks5Err := proxy.SOCKS5(network, socks5Addr, nil, proxy.Direct)
	if socks5Err != nil {
		log.Println(socks5Err)
		return nil
	}
	local, _ := ep.GetLocalAddress()
	targetAddr := fmt.Sprintf("%v:%d", local.Addr.To4(), local.Port)
	socks5Conn, connErr := dialer.Dial(network, targetAddr)
	if connErr != nil {
		log.Println(connErr)
		return nil
	}

	return &Tunnel{
		wq,
		ep,
		socks5Conn,
	}
}

func (tunnel *Tunnel) ReadFromLocalWriteToRemote() {
	defer tunnel.ep.Close()
	defer tunnel.socks5Conn.Close()

	// Create wait queue entry that notifies a channel.
	waitEntry, notifyCh := waiter.NewChannelEntry(nil)

	tunnel.wq.EventRegister(&waitEntry, waiter.EventIn)
	defer tunnel.wq.EventUnregister(&waitEntry)

	for {
		v, err := tunnel.ep.Read(nil)
		if err != nil {
			if err == tcpip.ErrWouldBlock {
				<-notifyCh
				continue
			}

			return
		}

		go tunnel.ReadFromRemoteWriteToLocal()
		tunnel.socks5Conn.Write(v)
	}
}

func (tunnel *Tunnel) ReadFromRemoteWriteToLocal() {
	defer tunnel.ep.Close()
	defer tunnel.socks5Conn.Close()

	buf := make([]byte, 1500)

	for {
		_, err := tunnel.socks5Conn.Read(buf)
		if err != nil {
			if err == io.EOF {
				log.Println("read eof from remote")
				return
			}

			log.Println(err) // use of closed network connection
			return
		}

		tunnel.ep.Write(buf, nil)
	}
}
