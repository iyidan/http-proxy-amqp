package pool

import (
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/ngaut/log"
	"github.com/streadway/amqp"

	"github.com/iyidan/http-proxy-amqp/config"
	"github.com/iyidan/http-proxy-amqp/util"
)

var (
	// ErrTooManyConn means opened connection num is overflow the config.maxConnections limit
	ErrTooManyConn = errors.New("pool: too many connections")

	// ErrBadConn occured when openChannel called on a closed channel or bad conn
	ErrBadConn = errors.New("pool: connection is bad")

	// ErrChannelNotAllClosed occured when the connection's channels not all closed
	// when close a connection, channels must be all closed
	ErrChannelNotAllClosed = errors.New("pool: connection's channels not all closed")

	// ErrNacked occured when send message not acked
	ErrNacked = errors.New("message not acked")

	// ErrPoolClosed occured when the pool was closed
	ErrPoolClosed = errors.New("pool closed")
)

// Connection represent a amqp real connection, which record the connection to user
type Connection struct {
	conn             *amqp.Connection
	l                sync.RWMutex
	numOpenedChannel int
}

func (conn *Connection) getNumOpenedChannel() int {
	conn.l.RLock()
	defer conn.l.RUnlock()
	return conn.numOpenedChannel
}

func (conn *Connection) openChannel() (*Channel, error) {
	conn.l.Lock()
	defer conn.l.Unlock()

	if conn.conn == nil {
		return nil, ErrBadConn
	}

	amqpCha, err := conn.conn.Channel()
	if err != nil {
		return nil, err
	}

	cha := &Channel{
		conn: conn,
		cha:  amqpCha,
	}

	// always in confirm mode
	if err := cha.cha.Confirm(false); err != nil {
		return nil, util.WrapError(err, "channel set to confirm mode failed")
	}
	cha.confirmCh = cha.cha.NotifyPublish(make(chan amqp.Confirmation, 1))

	conn.numOpenedChannel++
	return cha, nil
}

func (conn *Connection) decrNumOpenedChannel() {
	conn.l.Lock()
	defer conn.l.Unlock()
	conn.numOpenedChannel--
}

// close the amqp connection and put it to cache pool
func (conn *Connection) close(force bool) error {
	conn.l.Lock()
	defer conn.l.Unlock()

	if !force && conn.numOpenedChannel > 0 {
		return ErrChannelNotAllClosed
	}

	if conn.conn != nil {
		err := conn.conn.Close()
		if err != nil {
			log.Warnf("Connection.close: %s\n", err)
		}
	}
	conn.conn = nil
	return nil
}

// Channel represent a amqp channel, which expose connection to user
type Channel struct {
	conn      *Connection
	cha       *amqp.Channel
	confirmCh chan amqp.Confirmation
}

// close the amqp channel and decr it's connection numOpenedChannel
func (cha *Channel) close() {
	// not care about channel close error, because it's the client action
	cha.cha.Close()

	cha.conn.decrNumOpenedChannel()
	cha.cha = nil
	cha.conn = nil
}

// ConnPoolStats contains current pool states returned by call pool.Stats()
type ConnPoolStats struct {
	IdleChaNum int
	ConnNum    int
	BusyChaNum int32
	ReqChaNum  int
}

// ConnPool is the real connection pool
type ConnPool struct {
	conf  *config.Config
	conns []*Connection

	// defer close the unused connection to reduce pool lock time
	connDelayCloseCh chan *Connection
	connDelayClosed  chan struct{}

	l sync.Mutex

	reqChaList *ReqChaList

	// idle channels
	idleChas []*Channel

	chaBusyNum int32

	closed bool
}

// NewPool return a pool inited with the given config
func NewPool(conf *config.Config) *ConnPool {
	pool := &ConnPool{
		conf:  conf,
		conns: make([]*Connection, 0, conf.MaxConnections),

		connDelayCloseCh: make(chan *Connection, 10000),
		connDelayClosed:  make(chan struct{}),

		idleChas: make([]*Channel, 0, conf.MaxIdleChannels),

		reqChaList: &ReqChaList{},
	}

	go func() {
		for conn := range pool.connDelayCloseCh {
			err := conn.close(true)
			if err != nil {
				log.Warnf("conn.close: %s\n", err)
			}
		}
		close(pool.connDelayClosed)
	}()

	return pool
}

func (cop *ConnPool) incrChaBusyNum() {
	atomic.AddInt32(&cop.chaBusyNum, int32(1))
}

func (cop *ConnPool) decrChaBusyNum() {
	atomic.AddInt32(&cop.chaBusyNum, int32(-1))
}

func (cop *ConnPool) getChaBusyNum() int32 {
	return atomic.LoadInt32(&cop.chaBusyNum)
}

// GetConf get a copy of current config
func (cop *ConnPool) GetConf() config.Config {
	conf := *cop.conf
	return conf
}

// CloseAll close the whole connections and channels
func (cop *ConnPool) CloseAll() {
	cop.l.Lock()
	defer cop.l.Unlock()
	cop.close()
}

// close will close the pool and it's connections
func (cop *ConnPool) close() {
	if cop.closed {
		return
	}
	cop.closed = true

	// wait for connection all closed
	close(cop.connDelayCloseCh)
	<-cop.connDelayClosed

	for i, cha := range cop.idleChas {
		cha.close()
		cop.idleChas[i] = nil
	}
	cop.idleChas = nil

	for i, conn := range cop.conns {
		conn.close(true)
		cop.conns[i] = nil
	}
	cop.conns = nil
}

func (cop *ConnPool) removeConn(conn *Connection) error {

	if cop.conf.Debug {
		log.Debug("[conn] old conn closed")
	}

	foundIdx := -1
	for i := 0; i < len(cop.conns); i++ {
		if conn == cop.conns[i] {
			foundIdx = i
			break
		}
	}
	// delete from pool
	if foundIdx > -1 {
		copy(cop.conns[foundIdx:], cop.conns[foundIdx+1:])
		cop.conns[len(cop.conns)-1] = nil
		cop.conns = cop.conns[:len(cop.conns)-1]
	}

	// if pool closed , direct close the connection
	if cop.closed {
		if err := conn.close(true); err != nil {
			return err
		}
		return nil
	}

	// put into conn delay close channel
	cop.connDelayCloseCh <- conn
	return nil
}

func (cop *ConnPool) getConn() (*Connection, error) {
	if len(cop.conns) > 0 {
		for i := 0; i < len(cop.conns); i++ {
			// notice: the first connection may handel more channels
			if cop.conns[i].getNumOpenedChannel() < cop.conf.MaxChannelsPerConnection {
				return cop.conns[i], nil
			}
		}
	}

	if cop.conf.MaxConnections > 0 && len(cop.conns) >= cop.conf.MaxConnections {
		return nil, ErrTooManyConn
	}

	amqpConn, err := amqp.Dial(cop.conf.DSN)
	if err != nil {
		return nil, util.WrapError(err, "amqp.Dial")
	}
	conn := &Connection{conn: amqpConn, numOpenedChannel: 0}
	cop.conns = append(cop.conns, conn)

	if cop.conf.Debug {
		log.Debug("[conn] new conn opened")
	}

	return conn, nil
}

// Stats return current pool states
func (cop *ConnPool) Stats() *ConnPoolStats {
	cop.l.Lock()
	defer cop.l.Unlock()

	return &ConnPoolStats{
		IdleChaNum: len(cop.idleChas),
		ConnNum:    len(cop.conns),
		BusyChaNum: cop.getChaBusyNum(),
		ReqChaNum:  cop.reqChaList.Len(),
	}
}

func (cop *ConnPool) putChannel(cha *Channel) {

	cop.decrChaBusyNum()

	// if channel request is notified, skip put into idleChas
	if cop.reqChaList.NotifyOne(cha) {
		return
	}

	cop.l.Lock()
	lenFree := len(cop.idleChas)
	cop.l.Unlock()

	if lenFree >= cop.conf.MaxIdleChannels {
		cop.probeCloseChannel(cha)
		return
	}

	cop.l.Lock()
	cop.idleChas = append(cop.idleChas, cha)
	cop.l.Unlock()

}

// getChannel get a free channel from pool
func (cop *ConnPool) getChannel() (*Channel, error) {

GETFREECHANNEL:
	cop.l.Lock()

	if cop.closed {
		cop.l.Unlock()
		return nil, ErrPoolClosed
	}

	// step1: reuse free channels
	if len(cop.idleChas) > 0 {
		var cha *Channel
		// shift from free pool
		cha, cop.idleChas = cop.idleChas[0], cop.idleChas[1:]
		cop.incrChaBusyNum()

		cop.l.Unlock()
		return cha, nil
	}

	// step2: get connection
	conn, err := cop.getConn()
	if err == ErrTooManyConn {
		// unlock
		cop.l.Unlock()

		// wait for available channel
		// if wait return and has a free channel, use it
		ch := make(chan *Channel)
		cop.reqChaList.Put(ch)
		if cha := <-ch; cha != nil {
			cop.incrChaBusyNum()
			return cha, nil
		}

		// retry
		goto GETFREECHANNEL
	} else if err != nil {
		cop.close()
		// unlock
		cop.l.Unlock()
		util.FailOnError(err, "ConnPool.getChannel")
	}

	// step3: open new channel
	cha, err := conn.openChannel()

	if err == amqp.ErrClosed || err == ErrBadConn {
		log.Warnf("ConnPool.getChannel: %s", ErrBadConn)
		cop.removeConn(conn)
		// unlock
		cop.l.Unlock()
		goto GETFREECHANNEL
	} else if err == amqp.ErrChannelMax {
		log.Warnf("ConnPool.getChannel: %s", amqp.ErrChannelMax)
		cop.l.Unlock()
		goto GETFREECHANNEL
	} else if err != nil {
		cop.close()
		// unlock
		cop.l.Unlock()
		util.FailOnError(err, "ConnPool.getChannel")
	}

	cop.incrChaBusyNum()

	if cop.conf.Debug {
		log.Debug("[channel] new channel opened")
	}

	// unlock
	cop.l.Unlock()
	return cha, nil
}

func (cop *ConnPool) probeCloseChannel(cha *Channel) {
	conn := cha.conn
	cha.close()

	if cop.conf.Debug {
		log.Debug("[channel] old channel closed")
	}

	cop.l.Lock()
	defer cop.l.Unlock()
	if conn.getNumOpenedChannel() == 0 && len(cop.conns) > cop.conf.MinConnections {
		cop.removeConn(conn)
	}
}

// ConfirmSendMsg send message with confirm mode
func (cop *ConnPool) ConfirmSendMsg(exchange string, routingKey string, data []byte) error {

	if cop.conf.Debug {
		defer func() {
			stats, _ := json.Marshal(cop.Stats())
			conf, _ := json.Marshal(cop.conf)
			log.Debugf("debugStats: %s %s\n", stats, conf)
		}()
	}

	var err error
	var cha *Channel

	for i := 0; i < 5; i++ {
		cha, err = cop.getChannel()
		if err != nil {
			return err
		}
		err = cha.cha.Publish(
			exchange,   // exchange
			routingKey, // routing key
			true,       // mandatory，若为true，则当没有对应的队列，不ack
			false,      // immediate，若为true，则当没有消费者消费，不ack
			amqp.Publishing{
				ContentType:  "text/plain",
				Body:         data,
				DeliveryMode: 2, // 持久消息
			})
		if err == nil {
			break
		}
		cop.probeCloseChannel(cha)
	}

	if err != nil {
		return util.WrapError(err, "Failed to publish a message")
	}

	// waiting for the server confirm
	confirmed := <-cha.confirmCh

	// put current channel into idle pool
	cop.putChannel(cha)
	//cop.idleChaPutCh <- cha

	if confirmed.Ack {
		return nil
	}
	// @todo if channel closed by putChannel method, message would nacked
	log.Errorf("%s: %#v, %s, %x\n", ErrNacked, confirmed, data, data)
	return ErrNacked
}
