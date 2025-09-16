package http

import (
	"io"
	"net"
	nethttp "net/http"
	log "sw/ocpp/csms/internal/logging"
)

type TcpKeepAliveListener struct {
	*net.TCPListener
}

func ListenAndServeWithClose(addr string, handler nethttp.Handler) (io.Closer, error) {
	var (
		listener  net.Listener
		srvCloser io.Closer
		err       error
	)

	srv := &nethttp.Server{Addr: addr, Handler: handler}

	if addr == "" {
		addr = ":http"
	}

	listener, err = net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}

	go func() {
		err := srv.Serve(TcpKeepAliveListener{listener.(*net.TCPListener)})
		if err != nil {
			log.Logger.Errorf("HTTP Server Error - %s", err)
		}
	}()

	srvCloser = listener
	return srvCloser, nil
}
