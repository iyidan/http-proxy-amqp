package pool

import "sync"

// ReqChaList used for multi request waiting for some events
type ReqChaList struct {
	chs []chan *Channel
	l   sync.Mutex
}

// Len return current wait set length
func (rcl *ReqChaList) Len() int {
	rcl.l.Lock()
	defer rcl.l.Unlock()
	return len(rcl.chs)
}

// Put put a wait ch into the set
// Notice: ch is a channel and it's capacity is 1
func (rcl *ReqChaList) Put(ch chan *Channel) {
	rcl.l.Lock()
	defer rcl.l.Unlock()
	rcl.chs = append(rcl.chs, ch)
}

// NotifyOne notify the first wait chan
func (rcl *ReqChaList) NotifyOne(cha *Channel) bool {
	rcl.l.Lock()
	defer rcl.l.Unlock()
	if len(rcl.chs) > 0 {
		var ch chan *Channel
		ch, rcl.chs = rcl.chs[0], rcl.chs[1:]

		ch <- cha

		close(ch)
		return true
	}
	return false
}

// NotifyAll notify all wait chans
func (rcl *ReqChaList) NotifyAll() {
	rcl.l.Lock()
	defer rcl.l.Unlock()
	for i := 0; i < len(rcl.chs); i++ {
		close(rcl.chs[i])
	}
	rcl.chs = nil
}
