package apiserver

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"fmt"

	"strings"

	"github.com/iyidan/http-proxy-amqp/pool"
	"github.com/ngaut/log"
)

// InitServer init a new http server with the given pool
func InitServer(pool *pool.ConnPool) *http.Server {
	conf := pool.GetConf()

	mux := http.NewServeMux()

	// api for get pool stats
	mux.HandleFunc("/stats", func(res http.ResponseWriter, req *http.Request) {
		stats, _ := json.Marshal(pool.Stats())
		fmt.Fprintf(res, "%s", stats)
	})

	// api for confirm send message
	mux.HandleFunc("/confirm_send", func(res http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost && req.Method != http.MethodPut {
			fmt.Fprint(res, "method not allowed")
			return
		}

		exchange := strings.TrimSpace(req.URL.Query().Get("exchange"))
		routingKey := strings.TrimSpace(req.URL.Query().Get("routingKey"))

		if len(exchange) == 0 {
			fmt.Fprint(res, "exchange param empty")
			return
		}

		if len(routingKey) == 0 {
			fmt.Fprint(res, "exchange param empty")
			return
		}

		body, err := ioutil.ReadAll(req.Body)
		req.Body.Close()

		if err != nil {
			log.Errorf("read request body failed: %s\n", err)
			fmt.Fprintf(res, "read body fail: %s", err.Error())
			return
		}
		if len(body) == 0 {
			fmt.Fprint(res, "message body empty")
			return
		}

		err = pool.ConfirmSendMsg(exchange, routingKey, body)
		if err != nil {
			fmt.Fprintf(res, "ConfirmSendMsg error: %s", err)
			return
		}
		// ok when message is sent
		fmt.Fprint(res, "OK")
	})

	s := &http.Server{
		Addr:           conf.HTTPListenAddr,
		Handler:        mux,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	go func() {
		log.Error(s.ListenAndServe())
	}()

	return s
}
