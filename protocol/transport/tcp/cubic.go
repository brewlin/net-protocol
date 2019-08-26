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
	"math"
	"time"
)

// cubicState 存储与TCP CUBIC拥塞控制算法状态相关的变量。详见: https://tools.ietf.org/html/rfc8312.
type cubicState struct {
	// wLastMax is the previous wMax value.
	// 上次最大的拥塞窗口
	wLastMax float64

	// wMax is the value of the congestion window at the
	// time of last congestion event.
	// 上次拥塞时间时的拥塞窗口大小
	wMax float64

	// t denotes the time when the current congestion avoidance
	// was entered.
	// t 表进入拥塞避免阶段的时间。
	t time.Time

	// numCongestionEvents tracks the number of congestion events since last
	// RTO.
	// numCongestionEvents 跟踪自上次 RTO 以来的拥塞事件数。
	numCongestionEvents int

	// c is the cubic constant as specified in RFC8312. It's fixed at 0.4 as
	// per RFC.
	// 三次方函数的系数
	c float64

	// k is the time period that the above function takes to increase the
	// current window size to W_max if there are no further congestion
	// events and is calculated using the following equation:
	// k是在没有其他拥塞事件时将当前窗口大小增加到W_max所需的时间段，并使用以下公式计算：
	//
	// K = cubic_root(W_max*(1-beta_cubic)/C) (Eq. 2)
	k float64

	// beta is the CUBIC multiplication decrease factor. that is, when a
	// congestion event is detected, CUBIC reduces its cwnd to
	// W_cubic(0)=W_max*beta_cubic.
	// beta 是CUBIC乘法减少因子。也就是说，当检测到拥塞事件时，CUBIC将其cwnd减少到
	beta float64

	// wC is window computed by CUBIC at time t. It's calculated using the
	// formula:
	// wC 是由CUBIC在时间t计算的窗口。它使用公式计算：
	//
	//  W_cubic(t) = C*(t-K)^3 + W_max (Eq. 1)
	wC float64

	// wEst is the window computed by CUBIC at time t+RTT i.e
	// wEs t是CUBIC在时间 t+RTT 计算的窗口，即
	// W_cubic(t+RTT).
	wEst float64

	s *sender
}

// newCubicCC returns a partially initialized cubic state with the constants
// beta and c set and t set to current time.
// newCubicCC 返回部分初始化的 cubic 状态，常量为beta和c，t为当前时间。
func newCubicCC(s *sender) *cubicState {
	return &cubicState{
		t:    time.Now(),
		beta: 0.7,
		c:    0.4,
		s:    s,
	}
}

// enterCongestionAvoidance is used to initialize cubic in cases where we exit
// SlowStart without a real congestion event taking place. This can happen when
// a connection goes back to slow start due to a retransmit and we exceed the
// previously lowered ssThresh without experiencing packet loss.
//
// Refer: https://tools.ietf.org/html/rfc8312#section-4.8
// enterCongestionAvoidance 用于在我们退出 SlowStart 而没有发生真正的拥塞事件的情况下初始化 cubic。
// 当连接由于重新传输而返回慢启动时会发生这种情况，并且我们超过先前降低的ssThresh而不会遇到丢包。
func (c *cubicState) enterCongestionAvoidance() {
	// See: https://tools.ietf.org/html/rfc8312#section-4.7 &
	// https://tools.ietf.org/html/rfc8312#section-4.8
	// 初次进入拥塞避免，只记录当前拥塞避免的时间点，当前的窗口wMax=cwnd
	if c.numCongestionEvents == 0 {
		c.k = 0
		c.t = time.Now()
		c.wLastMax = c.wMax
		c.wMax = float64(c.s.sndCwnd)
	}
}

// updateSlowStart will update the congestion window as per the slow-start
// algorithm used by NewReno. If after adjusting the congestion window we cross
// the ssThresh then it will return the number of packets that must be consumed
// in congestion avoidance mode.
// updateSlowStart 将根据NewReno使用的慢启动算法更新拥塞窗口。
// 如果在调整拥塞窗口之后我们越过ssThresh，那么它将返回在拥塞避免模式下必须消耗的数据包的数量。
func (c *cubicState) updateSlowStart(packetsAcked int) int {
	// Don't let the congestion window cross into the congestion
	// avoidance range.
	newcwnd := c.s.sndCwnd + packetsAcked
	enterCA := false
	if newcwnd >= c.s.sndSsthresh {
		newcwnd = c.s.sndSsthresh
		c.s.sndCAAckCount = 0
		enterCA = true
	}

	packetsAcked -= newcwnd - c.s.sndCwnd
	c.s.sndCwnd = newcwnd
	if enterCA {
		// 进入拥塞避免
		c.enterCongestionAvoidance()
	}
	return packetsAcked
}

// Update updates cubic's internal state variables. It must be called on every
// ACK received.
// Refer: https://tools.ietf.org/html/rfc8312#section-4
// Update 更新 cubic 内部状态变量，每次收到ack都必须调用
func (c *cubicState) Update(packetsAcked int) {
	if c.s.sndCwnd < c.s.sndSsthresh {
		packetsAcked = c.updateSlowStart(packetsAcked)
		if packetsAcked == 0 {
			return
		}
	} else {
		// 拥塞避免阶段
		c.s.rtt.Lock()
		srtt := c.s.rtt.srtt
		c.s.rtt.Unlock()
		c.s.sndCwnd = c.getCwnd(packetsAcked, c.s.sndCwnd, srtt)
	}
}

// cubicCwnd computes the CUBIC congestion window after t seconds from last
// congestion event.
// cubicCwnd 在上次拥塞事件发生t秒后计算CUBIC拥塞窗口。
func (c *cubicState) cubicCwnd(t float64) float64 {
	return c.c*math.Pow(t, 3.0) + c.wMax
}

// getCwnd returns the current congestion window as computed by CUBIC.
// Refer: https://tools.ietf.org/html/rfc8312#section-4
// getCwnd 返回由CUBIC计算的当前拥塞窗口。
func (c *cubicState) getCwnd(packetsAcked, sndCwnd int, srtt time.Duration) int {
	elapsed := time.Since(c.t).Seconds()

	// Compute the window as per Cubic after 'elapsed' time
	// since last congestion event.
	c.wC = c.cubicCwnd(elapsed - c.k)

	// Compute the TCP friendly estimate of the congestion window.
	c.wEst = c.wMax*c.beta + (3.0*((1.0-c.beta)/(1.0+c.beta)))*(elapsed/srtt.Seconds())

	// Make sure in the TCP friendly region CUBIC performs at least
	// as well as Reno.
	if c.wC < c.wEst && float64(sndCwnd) < c.wEst {
		// TCP Friendly region of cubic.
		return int(c.wEst)
	}

	// In Concave/Convex region of CUBIC, calculate what CUBIC window
	// will be after 1 RTT and use that to grow congestion window
	// for every ack.
	tEst := (time.Since(c.t) + srtt).Seconds()
	wtRtt := c.cubicCwnd(tEst - c.k)
	// As per 4.3 for each received ACK cwnd must be incremented
	// by (w_cubic(t+RTT) - cwnd/cwnd.
	cwnd := float64(sndCwnd)
	for i := 0; i < packetsAcked; i++ {
		// Concave/Convex regions of cubic have the same formulas.
		// See: https://tools.ietf.org/html/rfc8312#section-4.3
		cwnd += (wtRtt - cwnd) / cwnd
	}
	return int(cwnd)
}

// HandleNDupAcks implements congestionControl.HandleNDupAcks.
// 收到三次重复ack，调用 HandleNDupAcks
func (c *cubicState) HandleNDupAcks() {
	// See: https://tools.ietf.org/html/rfc8312#section-4.5
	c.numCongestionEvents++
	c.t = time.Now()
	c.wLastMax = c.wMax
	c.wMax = float64(c.s.sndCwnd)

	c.fastConvergence()
	c.reduceSlowStartThreshold()
}

// HandleRTOExpired implements congestionContrl.HandleRTOExpired.
// 发生重传时调用 HandleRTOExpired。
func (c *cubicState) HandleRTOExpired() {
	// See: https://tools.ietf.org/html/rfc8312#section-4.6
	c.t = time.Now()
	c.numCongestionEvents = 0
	c.wLastMax = c.wMax
	c.wMax = float64(c.s.sndCwnd)

	c.fastConvergence()

	// We lost a packet, so reduce ssthresh.
	c.reduceSlowStartThreshold()

	// Reduce the congestion window to 1, i.e., enter slow-start. Per
	// RFC 5681, page 7, we must use 1 regardless of the value of the
	// initial congestion window.
	c.s.sndCwnd = 1
}

// fastConvergence implements the logic for Fast Convergence algorithm as
// described in https://tools.ietf.org/html/rfc8312#section-4.6.
// 快速收敛
func (c *cubicState) fastConvergence() {
	if c.wMax < c.wLastMax {
		c.wLastMax = c.wMax
		c.wMax = c.wMax * (1.0 + c.beta) / 2.0
	} else {
		c.wLastMax = c.wMax
	}
	// Recompute k as wMax may have changed.
	c.k = math.Cbrt(c.wMax * (1 - c.beta) / c.c)
}

// PostRecovery implemements congestionControl.PostRecovery.
// 更新t为当前的时间，当发送方退出快速恢复阶段时，将调用 PostRecovery
func (c *cubicState) PostRecovery() {
	c.t = time.Now()
}

// reduceSlowStartThreshold returns new SsThresh as described in
// https://tools.ietf.org/html/rfc8312#section-4.7.
// 按 cubic 的算法更新慢启动和拥塞避免之间的阈值
func (c *cubicState) reduceSlowStartThreshold() {
	c.s.sndSsthresh = int(math.Max(float64(c.s.sndCwnd)*c.beta, 2.0))
}
