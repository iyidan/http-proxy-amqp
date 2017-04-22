package main

// This is a amqp connection pool implementation
// app(such as php...) <--> short connection <--> http2amqp <--> long life connection <--> amqp

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"time"

	"github.com/iyidan/http-proxy-amqp/apiserver"
	"github.com/iyidan/http-proxy-amqp/config"
	"github.com/iyidan/http-proxy-amqp/pool"
	"github.com/ngaut/log"
)

func main() {

	sc := make(chan os.Signal, 1)
	signal.Notify(sc,
		os.Kill,
		os.Interrupt,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	flag.Parse()

	conf := config.InitConfig()
	connPool := pool.NewPool(conf)

	srv := apiserver.InitServer(connPool)

	srv.Shutdown(context.Background())

	log.Infof("server started\n with conf: %#v\n", *conf)

	s := <-sc
	log.Errorf("main: received signal: %v\n", s)

	// graceful shutdown server
	timeout := time.Second * 3
	ctx, cancelf := context.WithTimeout(context.Background(), timeout)
	srv.Shutdown(ctx)
	select {
	case <-ctx.Done():
		log.Info("server closed")
	case <-time.After(timeout):
		cancelf()
		<-ctx.Done()
	}
	if ctx.Err() != nil {
		log.Errorf("Shutdown server error: %s\n", ctx.Err())
	}

	// close pool
	connPool.CloseAll()
	log.Info("connPool closed")
}
