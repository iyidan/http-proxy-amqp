package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ngaut/log"
	"github.com/streadway/amqp"

	"flag"

	"github.com/iyidan/http-proxy-amqp/config"
	"github.com/iyidan/http-proxy-amqp/pool"
)

var (
	testPool       *pool.ConnPool
	testExchange   = "amq.topic"
	testRoutingKey = "test.topic.routingKey"

	flagTestMsgNum     = flag.Int("msgnum", 20000, "num message for test")
	flagTestPrintStats = flag.Bool("vv", false, "print pool stats after msg send")

	testConf *config.Config
)

// initPool init a pool for test
// pass the command-line args such as "go test -args -config=xxx.conf.json"
func initTestConf() {
	testConf = config.InitConfig()
}

func printStats(testPool *pool.ConnPool) {
	s, _ := json.Marshal(testPool.Stats())
	c, _ := json.Marshal(testPool.GetConf())
	fmt.Printf("stats: %s, conf: %s\n", s, c)
}

// conf: current benchmark conf
// c: client num
// n: total request num
// msg: send message
func benchmarkConfirmSendMsg(testPool *pool.ConnPool, c, n int, msg []byte) int {

	conf := testPool.GetConf()
	if conf.Debug {
		fmt.Printf("BEFORE_CONF: %#v\n", testPool.GetConf())
	}

	npc := n / c
	leftn := n % c
	w := &sync.WaitGroup{}

	startT := time.Now()

	for i := 0; i < c; i++ {
		w.Add(1)
		sendNum := npc
		if i == 0 {
			sendNum += leftn
		}
		go func(sendNum int) {
			defer w.Done()
			for j := 0; j < sendNum; j++ {
				startT := time.Now()
				if conf.Debug {
					err := testPool.ConfirmSendMsg(testExchange, testRoutingKey, append(msg, []byte(startT.String())...))
					if err != nil {
						panic(err)
					}
					endT := time.Now()
					fmt.Printf("cost: %s\n", endT.Sub(startT))
					continue
				}
				err := testPool.ConfirmSendMsg(testExchange, testRoutingKey, append(msg, []byte(startT.String())...))
				if err != nil {
					panic(err)
				}
			}
		}(sendNum)
	}
	w.Wait()

	stopT := time.Now()

	if conf.Debug {
		fmt.Printf("END_CONF: %#v\n", testPool.GetConf())
	}

	tps := float64(n) / stopT.Sub(startT).Seconds()
	fmt.Printf("tps: %v/s, c: %d, n: %d\n", tps, c, n)

	return c * npc
}

func BenchmarkOriginConfirmSendMsgOneChannel(b *testing.B) {
	conf := *testConf
	conn, err := amqp.Dial(conf.DSN)
	if err != nil {
		b.Fatal(err)
	}

	cha, err := conn.Channel()
	if err != nil {
		b.Fatal(err)
	}

	// always in confirm mode
	if err := cha.Confirm(false); err != nil {
		b.Fatal(err)
	}
	confirmCh := cha.NotifyPublish(make(chan amqp.Confirmation, 1))

	n := *flagTestMsgNum
	data := []byte("hello, amqp!")

	startT := time.Now()
	for i := 0; i < n; i++ {
		// send msg
		err := cha.Publish(
			testExchange,   // exchange
			testRoutingKey, // routing key
			true,           // mandatory，若为true，则当没有对应的队列，不ack
			false,          // immediate，若为true，则当没有消费者消费，不ack
			amqp.Publishing{
				ContentType:  "text/plain",
				Body:         data,
				DeliveryMode: 2, // 持久消息
			})
		if err != nil {
			b.Fatal(err)
		}
		// waiting for the server confirm
		confirmed := <-confirmCh
		if !confirmed.Ack {
			log.Fatal("msg not confirmed")
		}
	}

	stopT := time.Now()

	tps := float64(n) / stopT.Sub(startT).Seconds()
	fmt.Printf("tps: %v/s, c: %d, n: %d\n", tps, 1, n)

	cha.Close()
	conn.Close()
	time.Sleep(time.Second * 5)
}

func BenchmarkConfirmSendMsgAlwaysNewConn(b *testing.B) {

	conf := *testConf
	conf.MaxChannelsPerConnection = 1
	conf.MaxConnections = 100
	conf.MaxIdleChannels = 1
	conf.Debug = *flagTestPrintStats

	c := 200
	n := *flagTestMsgNum

	testPool := pool.NewPool(&conf)
	benchmarkConfirmSendMsg(testPool, c, n, []byte("hello, pool!"))
	time.Sleep(time.Second * 1)
	testPool.CloseAll()

	time.Sleep(time.Second * 5)
}

func BenchmarkConfirmSendMsg(b *testing.B) {

	conf := *testConf
	conf.MaxChannelsPerConnection = 500
	conf.MaxConnections = 100
	conf.MaxIdleChannels = 500
	conf.MinConnections = 5
	conf.Debug = *flagTestPrintStats

	c := 200
	n := *flagTestMsgNum

	testPool := pool.NewPool(&conf)
	benchmarkConfirmSendMsg(testPool, c, n, []byte("hello, pool!"))
	time.Sleep(time.Second * 1)
	testPool.CloseAll()

	time.Sleep(time.Second * 5)
}

func init() {
	initTestConf()
}
