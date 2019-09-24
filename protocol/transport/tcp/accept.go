// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tcp

import (
	"encoding/binary"
	"hash"
	"io"
	"sync"
	"time"

	"crypto/sha1"

	"github.com/brewlin/net-protocol/pkg/rand"
	"github.com/brewlin/net-protocol/pkg/seqnum"
	"github.com/brewlin/net-protocol/pkg/sleep"
	"github.com/brewlin/net-protocol/pkg/waiter"
	tcpip "github.com/brewlin/net-protocol/protocol"
	"github.com/brewlin/net-protocol/protocol/header"
	"github.com/brewlin/net-protocol/stack"
)

const (
	// tsLen is the length, in bits, of the timestamp in the SYN cookie.
	tsLen = 8

	// tsMask is a mask for timestamp values (i.e., tsLen bits).
	tsMask = (1 << tsLen) - 1

	// tsOffset is the offset, in bits, of the timestamp in the SYN cookie.
	tsOffset = 24

	// hashMask is the mask for hash values (i.e., tsOffset bits).
	hashMask = (1 << tsOffset) - 1

	// maxTSDiff is the maximum allowed difference between a received cookie
	// timestamp and the current timestamp. If the difference is greater
	// than maxTSDiff, the cookie is expired.
	maxTSDiff = 2
)

var (
	// SynRcvdCountThreshold is the global maximum number of connections
	// that are allowed to be in SYN-RCVD state before TCP starts using SYN
	// cookies to accept connections.
	//
	// It is an exported variable only for testing, and should not otherwise
	// be used by importers of this package.
	SynRcvdCountThreshold uint64 = 1000

	// mssTable is a slice containing the possible MSS values that we
	// encode in the SYN cookie with two bits.
	mssTable = []uint16{536, 1300, 1440, 1460}
)

func encodeMSS(mss uint16) uint32 {
	for i := len(mssTable) - 1; i > 0; i-- {
		if mss >= mssTable[i] {
			return uint32(i)
		}
	}
	return 0
}

// syncRcvdCount is the number of endpoints in the SYN-RCVD state. The value is
// protected by a mutex so that we can increment only when it's guaranteed not
// to go above a threshold.
var synRcvdCount struct {
	sync.Mutex
	value   uint64
	pending sync.WaitGroup
}

// listenContext is used by a listening endpoint to store state used while
// listening for connections. This struct is allocated by the listen goroutine
// and must not be accessed or have its methods called concurrently as they
// may mutate the stored objects.
type listenContext struct {
	stack  *stack.Stack
	rcvWnd seqnum.Size
	nonce  [2][sha1.BlockSize]byte

	hasherMu sync.Mutex
	hasher   hash.Hash
	v6only   bool
	netProto tcpip.NetworkProtocolNumber
}

// timeStamp returns an 8-bit timestamp with a granularity of 64 seconds.
func timeStamp() uint32 {
	return uint32(time.Now().Unix()>>6) & tsMask
}

// incSynRcvdCount tries to increment the global number of endpoints in SYN-RCVD
// state. It succeeds if the increment doesn't make the count go beyond the
// threshold, and fails otherwise.
func incSynRcvdCount() bool {
	synRcvdCount.Lock()
	defer synRcvdCount.Unlock()

	if synRcvdCount.value >= SynRcvdCountThreshold {
		return false
	}

	synRcvdCount.pending.Add(1)
	synRcvdCount.value++

	return true
}

// decSynRcvdCount atomically decrements the global number of endpoints in
// SYN-RCVD state. It must only be called if a previous call to incSynRcvdCount
// succeeded.
func decSynRcvdCount() {
	synRcvdCount.Lock()
	defer synRcvdCount.Unlock()

	synRcvdCount.value--
	synRcvdCount.pending.Done()
}

// newListenContext creates a new listen context.
func newListenContext(stack *stack.Stack, rcvWnd seqnum.Size, v6only bool, netProto tcpip.NetworkProtocolNumber) *listenContext {
	l := &listenContext{
		stack:    stack,
		rcvWnd:   rcvWnd,
		hasher:   sha1.New(),
		v6only:   v6only,
		netProto: netProto,
	}

	rand.Read(l.nonce[0][:])
	rand.Read(l.nonce[1][:])

	return l
}

// cookieHash calculates the cookieHash for the given id, timestamp and nonce
// index. The hash is used to create and validate cookies.
func (l *listenContext) cookieHash(id stack.TransportEndpointID, ts uint32, nonceIndex int) uint32 {

	// Initialize block with fixed-size data: local ports and v.
	var payload [8]byte
	binary.BigEndian.PutUint16(payload[0:], id.LocalPort)
	binary.BigEndian.PutUint16(payload[2:], id.RemotePort)
	binary.BigEndian.PutUint32(payload[4:], ts)

	// Feed everything to the hasher.
	l.hasherMu.Lock()
	l.hasher.Reset()
	l.hasher.Write(payload[:])
	l.hasher.Write(l.nonce[nonceIndex][:])
	io.WriteString(l.hasher, string(id.LocalAddress))
	io.WriteString(l.hasher, string(id.RemoteAddress))

	// Finalize the calculation of the hash and return the first 4 bytes.
	h := make([]byte, 0, sha1.Size)
	h = l.hasher.Sum(h)
	l.hasherMu.Unlock()

	return binary.BigEndian.Uint32(h[:])
}

// createCookie creates a SYN cookie for the given id and incoming sequence
// number.
func (l *listenContext) createCookie(id stack.TransportEndpointID, seq seqnum.Value, data uint32) seqnum.Value {
	ts := timeStamp()
	v := l.cookieHash(id, 0, 0) + uint32(seq) + (ts << tsOffset)
	v += (l.cookieHash(id, ts, 1) + data) & hashMask
	return seqnum.Value(v)
}

// isCookieValid checks if the supplied cookie is valid for the given id and
// sequence number. If it is, it also returns the data originally encoded in the
// cookie when createCookie was called.
func (l *listenContext) isCookieValid(id stack.TransportEndpointID, cookie seqnum.Value, seq seqnum.Value) (uint32, bool) {
	ts := timeStamp()
	v := uint32(cookie) - l.cookieHash(id, 0, 0) - uint32(seq)
	cookieTS := v >> tsOffset
	if ((ts - cookieTS) & tsMask) > maxTSDiff {
		return 0, false
	}

	return (v - l.cookieHash(id, cookieTS, 1)) & hashMask, true
}

// createConnectedEndpoint creates a new connected endpoint, with the connection
// parameters given by the arguments.
func (l *listenContext) createConnectedEndpoint(s *segment, iss seqnum.Value, irs seqnum.Value, rcvdSynOpts *header.TCPSynOptions) (*endpoint, *tcpip.Error) {
	// Create a new endpoint.
	netProto := l.netProto
	if netProto == 0 {
		netProto = s.route.NetProto
	}
	n := newEndpoint(l.stack, netProto, nil)
	n.v6only = l.v6only
	n.id = s.id
	n.boundNICID = s.route.NICID()
	n.route = s.route.Clone()
	n.effectiveNetProtos = []tcpip.NetworkProtocolNumber{s.route.NetProto}
	n.rcvBufSize = int(l.rcvWnd)

	n.maybeEnableTimestamp(rcvdSynOpts)
	n.maybeEnableSACKPermitted(rcvdSynOpts)

	// Register new endpoint so that packets are routed to it.
	if err := n.stack.RegisterTransportEndpoint(n.boundNICID, n.effectiveNetProtos, ProtocolNumber, n.id, n); err != nil {
		n.Close()
		return nil, err
	}

	n.isRegistered = true
	n.state = stateConnected

	// Create sender and receiver.
	//
	// The receiver at least temporarily has a zero receive window scale,
	// but the caller may change it (before starting the protocol loop).
	n.snd = newSender(n, iss, irs, s.window, rcvdSynOpts.MSS, rcvdSynOpts.WS)
	n.rcv = newReceiver(n, irs, l.rcvWnd, 0)

	return n, nil
}

// createEndpoint creates a new endpoint in connected state and then performs
// the TCP 3-way handshake.
func (l *listenContext) createEndpointAndPerformHandshake(s *segment, opts *header.TCPSynOptions) (*endpoint, *tcpip.Error) {
	// Create new endpoint.
	irs := s.sequenceNumber
	cookie := l.createCookie(s.id, irs, encodeMSS(opts.MSS))
	ep, err := l.createConnectedEndpoint(s, cookie, irs, opts)
	if err != nil {
		return nil, err
	}

	// Perform the 3-way handshake.
	// 执行三次握手
	h, err := newHandshake(ep, l.rcvWnd)
	if err != nil {
		ep.Close()
		return nil, err
	}

	// 标记状态为 handshakeSynRcvd 和 h.flags为 syn+ack
	h.resetToSynRcvd(cookie, irs, opts)
	if err := h.execute(); err != nil {
		ep.Close()
		return nil, err
	}

	// Update the receive window scaling. We can't do it before the
	// handshake because it's possible that the peer doesn't support window
	// scaling.
	ep.rcv.rcvWndScale = h.effectiveRcvWndScale()

	return ep, nil
}

// deliverAccepted delivers the newly-accepted endpoint to the listener. If the
// endpoint has transitioned out of the listen state, the new endpoint is closed
// instead.
func (e *endpoint) deliverAccepted(n *endpoint) {
	e.mu.RLock()
	if e.state == stateListen {
		e.acceptedChan <- n
		e.waiterQueue.Notify(waiter.EventIn)
	} else {
		n.Close()
	}
	e.mu.RUnlock()
}

// handleSynSegment is called in its own goroutine once the listening endpoint
// receives a SYN segment. It is responsible for completing the handshake and
// queueing the new endpoint for acceptance.
//
// A limited number of these goroutines are allowed before TCP starts using SYN
// cookies to accept connections.
// 一旦侦听端点收到SYN段，handleSynSegment就会在其自己的goroutine中调用。它负责完成握手并将新端点排队以进行接受。
// 在TCP开始使用SYN cookie接受连接之前，允许使用有限数量的这些goroutine。
func (e *endpoint) handleSynSegment(ctx *listenContext, s *segment, opts *header.TCPSynOptions) {
	defer decSynRcvdCount()
	defer s.decRef()

	n, err := ctx.createEndpointAndPerformHandshake(s, opts)
	if err != nil {
		return
	}
	// 到这里，三次握手已经完成，那么分发一个新的连接
	e.deliverAccepted(n)
}

// handleListenSegment is called when a listening endpoint receives a segment
// and needs to handle it.
func (e *endpoint) handleListenSegment(ctx *listenContext, s *segment) {
	switch s.flags {
	case flagSyn:
		// syn报文处理
		// 分析tcp选项
		opts := parseSynSegmentOptions(s)
		// 如果处于 syn-recv 状态的连接没有超过 1000，那么不需要生成 cookie，
		// 而是直接分配和新建连接。
		if incSynRcvdCount() {
			s.incRef()
			go e.handleSynSegment(ctx, s, &opts)
		} else {
			cookie := ctx.createCookie(s.id, s.sequenceNumber, encodeMSS(opts.MSS))
			// Send SYN with window scaling because we currently
			// dont't encode this information in the cookie.
			//
			// Enable Timestamp option if the original syn did have
			// the timestamp option specified.
			synOpts := header.TCPSynOptions{
				WS:    -1,
				TS:    opts.TS,
				TSVal: tcpTimeStamp(timeStampOffset()),
				TSEcr: opts.TSVal,
			}
			// 返回 syn+ack 报文
			sendSynTCP(&s.route, s.id, flagSyn|flagAck, cookie, s.sequenceNumber+1, ctx.rcvWnd, synOpts)
		}

	case flagAck:
		// 三次握手最后一次 ack 报文
		if data, ok := ctx.isCookieValid(s.id, s.ackNumber-1, s.sequenceNumber-1); ok && int(data) < len(mssTable) {
			// Create newly accepted endpoint and deliver it.
			rcvdSynOptions := &header.TCPSynOptions{
				MSS: mssTable[data],
				// Disable Window scaling as original SYN is
				// lost.
				WS: -1,
			}
			// When syn cookies are in use we enable timestamp only
			// if the ack specifies the timestamp option assuming
			// that the other end did in fact negotiate the
			// timestamp option in the original SYN.
			if s.parsedOptions.TS {
				rcvdSynOptions.TS = true
				rcvdSynOptions.TSVal = s.parsedOptions.TSVal
				rcvdSynOptions.TSEcr = s.parsedOptions.TSEcr
			}

			// 三次握手已经完成，新建一个tcp连接
			n, err := ctx.createConnectedEndpoint(s, s.ackNumber-1, s.sequenceNumber-1, rcvdSynOptions)
			if err == nil {
				// clear the tsOffset for the newly created
				// endpoint as the Timestamp was already
				// randomly offset when the original SYN-ACK was
				// sent above.
				n.tsOffset = 0
				e.deliverAccepted(n)
			}
		}
	}
}

// protocolListenLoop is the main loop of a listening TCP endpoint. It runs in
// its own goroutine and is responsible for handling connection requests.
// tcp 监听主流程
// protocolListenLoop 是侦听TCP端点的主循环。它在自己的goroutine中运行，负责处理连接请求。
func (e *endpoint) protocolListenLoop(rcvWnd seqnum.Size) *tcpip.Error {
	//事件循环后退出前做的收尾工作
	defer func() {
		// Mark endpoint as closed. This will prevent goroutines running
		// handleSynSegment() from attempting to queue new connections
		// to the endpoint.
		e.mu.Lock()
		e.state = stateClosed

		// Do cleanup if needed.
		e.completeWorkerLocked()

		if e.drainDone != nil {
			close(e.drainDone)
		}
		e.mu.Unlock()

		// Notify waiters that the endpoint is shutdown.
		e.waiterQueue.Notify(waiter.EventIn | waiter.EventOut)
	}()

	e.mu.Lock()
	v6only := e.v6only
	e.mu.Unlock()

	ctx := newListenContext(e.stack, rcvWnd, v6only, e.netProto)

	// 事件触发器的初始化和事件的添加
	s := sleep.Sleeper{}
	s.AddWaker(&e.notificationWaker, wakerForNotification)
	s.AddWaker(&e.newSegmentWaker, wakerForNewSegment)
	// for循环拿到事件index，然后进行相应的处理
	for {
		switch index, _ := s.Fetch(true); index {
		case wakerForNotification:
			n := e.fetchNotifications()
			if n&notifyClose != 0 {
				return nil
			}
			if n&notifyDrain != 0 {
				for !e.segmentQueue.empty() {
					s := e.segmentQueue.dequeue()
					e.handleListenSegment(ctx, s)
					s.decRef()
				}
				synRcvdCount.pending.Wait()
				close(e.drainDone)
				<-e.undrain
			}

		case wakerForNewSegment:
			// Process at most maxSegmentsPerWake segments.
			// 接收和处理tcp报文
			mayRequeue := true
			for i := 0; i < maxSegmentsPerWake; i++ {
				s := e.segmentQueue.dequeue()
				if s == nil {
					mayRequeue = false
					break
				}

				// 处理tcp数据段 s
				e.handleListenSegment(ctx, s)
				s.decRef()
			}

			// If the queue is not empty, make sure we'll wake up
			// in the next iteration.
			if mayRequeue && !e.segmentQueue.empty() {
				e.newSegmentWaker.Assert()
			}
		}
	}
}
