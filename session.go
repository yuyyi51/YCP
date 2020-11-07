package YCP

import (
	"bytes"
	"code.int-2.me/yuyyi51/YCP/internal"
	"code.int-2.me/yuyyi51/YCP/packet"
	"code.int-2.me/yuyyi51/YCP/utils"
	"code.int-2.me/yuyyi51/ylog"
	"container/list"
	"fmt"
	"io"
	"math/rand"
	"net"
	"sync"
	"time"
)

type Session struct {
	conv uint32

	conn            net.PacketConn
	remoteAddr      net.Addr
	receivedPackets chan *packet.Packet
	sentPackets     chan *packet.Packet
	sessMux         *sync.RWMutex

	writeBuffer    []byte
	writeBufferLen int
	writingMux     *sync.RWMutex
	writeSignal    chan struct{}

	dataBuffer []byte
	readingMux *sync.RWMutex
	readSignal chan struct{}

	dataManager *internal.DataManager
	ackManager  *ackManager
	closeSignal chan struct{}
	dataSignal  chan struct{}
	ackChan     chan struct{}

	congestion internal.CongestionAlgorithm

	nextPktSeq     uint64
	nextDataOffset uint64

	history *internal.PacketHistory

	needAck            bool
	receiveFirstPacket bool

	rttStat internal.RttStat

	logger              ylog.ILogger
	retransmissionQueue []packet.Packet

	lossRate    int
	closeLocal  bool
	finSent     bool
	closeRemote bool
}

func NewSession(conn net.PacketConn, addr net.Addr, conv uint32, logger ylog.ILogger) *Session {
	session := &Session{
		conv:            conv,
		conn:            conn,
		remoteAddr:      addr,
		receivedPackets: make(chan *packet.Packet, 500),
		dataManager:     internal.NewDataManager(logger),
		ackManager:      newAckManager(),
		sessMux:         new(sync.RWMutex),
		writeBuffer:     make([]byte, packet.SessionWriteBufferSize),
		writingMux:      new(sync.RWMutex),
		writeSignal:     make(chan struct{}, 1),
		closeSignal:     make(chan struct{}, 1),
		dataSignal:      make(chan struct{}, 1),
		readSignal:      make(chan struct{}, 1),
		congestion:      internal.NewBbrSender(logger),
		readingMux:      new(sync.RWMutex),
		history:         internal.NewPacketHistory(logger),
		ackChan:         make(chan struct{}, 1),
		logger:          logger,
		sentPackets:     make(chan *packet.Packet, 500),
	}
	go session.run()
	go session.senderFunc()
	return session
}

func (sess *Session) senderFunc() {
	for {
		pkt, ok := <-sess.sentPackets
		if !ok {
			break
		}
		_, _ = sess.conn.WriteTo(pkt.Pack(), sess.remoteAddr)
	}
}

func (sess *Session) Read(b []byte) (n int, err error) {
	sess.readingMux.Lock()
	defer sess.readingMux.Unlock()
	remain := len(b)
	offset := 0
	if len(sess.dataBuffer) != 0 {
		if len(b) >= len(sess.dataBuffer) {
			copy(b, sess.dataBuffer)
			remain -= len(sess.dataBuffer)
			offset += len(sess.dataBuffer)
			sess.dataBuffer = nil
		} else {
			copy(b, sess.dataBuffer[:len(b)])
			sess.dataBuffer = sess.dataBuffer[len(b):]
			return len(b), nil
		}
	}
	if remain == 0 {
		return len(b), nil
	}
	for {
		data, fin := sess.dataManager.PopData()
		if fin {
			err = io.EOF
			break
		}
		if remain >= len(data) {
			copy(b[offset:], data)
			offset += len(data)
		} else {
			copy(b[offset:], data[:remain])
			sess.dataBuffer = data[remain:]
			offset += remain
			break
		}
		sess.logger.Debug("Reading data: %d, offset: %d, dataBuffer: %d", len(data), offset, len(sess.dataBuffer))
		if offset == 0 {
			// no data
			select {
			case <-sess.readSignal:
			}
		} else {
			break
		}
	}
	sess.logger.Debug("Read data done, offset: %d, err %v", offset, err)
	return offset, err
}

func (sess *Session) Write(b []byte) (n int, err error) {
	sess.writingMux.Lock()
	defer sess.writingMux.Unlock()
	sess.sessMux.Lock()
	sess.logger.Debug("start Write, len %d, writeBuffer: %d", len(b), sess.writeBufferLen)
	if len(b)+sess.writeBufferLen <= len(sess.writeBuffer) {
		// enough space in buffer
		copy(sess.writeBuffer[sess.writeBufferLen:], b)
		sess.writeBufferLen += len(b)
		sess.SignalData()
		sess.sessMux.Unlock()
		sess.logger.Debug("finish Write, bytesWritten: %d, writeBuffer: %d", len(b), sess.writeBufferLen)
		return len(b), nil
	}
	n = 0
	for {
		canCopy := len(sess.writeBuffer) - sess.writeBufferLen
		if canCopy > len(b)-n {
			canCopy = len(b) - n
		}
		if canCopy > 0 {
			copy(sess.writeBuffer[sess.writeBufferLen:], b[n:n+canCopy])
			n += canCopy
			sess.writeBufferLen += canCopy
			//sess.SignalData()
		}
		sess.logger.Debug("Writing data, canCopy: %d, dataWritten: %d, dataRemain: %d, dataLen: %d, writeBuffer: %d", canCopy, n, len(b)-n, len(b), sess.writeBufferLen)
		if n >= len(b) {
			sess.sessMux.Unlock()
			break
		}
		sess.sessMux.Unlock()
		select {
		case <-sess.writeSignal:

		case <-sess.closeSignal:
			err = fmt.Errorf("connection closed")
			break
		}
		sess.sessMux.Lock()
	}
	sess.logger.Debug("finish Write, bytesWritten: %d, writeBuffer: %d", n, sess.writeBufferLen)
	return
}

func (sess *Session) Close() error {
	sess.closeLocal = true
	sess.SignalClose()
	return nil
}

func (sess *Session) LocalAddr() net.Addr {
	return sess.conn.LocalAddr()
}

func (sess *Session) RemoteAddr() net.Addr {
	return sess.remoteAddr
}

func (sess *Session) SetDeadline(t time.Time) error {
	return nil
}

func (sess *Session) SetReadDeadline(t time.Time) error {
	return nil
}

func (sess *Session) SetWriteDeadline(t time.Time) error {
	return nil
}

func (sess *Session) String() string {
	return fmt.Sprintf("[%d]%s", sess.conv, sess.RemoteAddr())
}

func (sess *Session) run() {
	sendTimer := time.NewTimer(packet.SessionSendInterval)
	closeTimer := time.NewTimer(time.Hour * 24 * 3650)
	closeSet := false
loop:
	for {
		select {
		case pkt := <-sess.receivedPackets:
			sess.handlePacket(pkt)
			if !sess.receiveFirstPacket {
				sess.receiveFirstPacket = true
				// send ack for the first packet soon to probe the rtt
				sess.sendAck()
			}
		default:
		}

		select {
		case pkt := <-sess.receivedPackets:
			sess.handlePacket(pkt)
			if !sess.receiveFirstPacket {
				sess.receiveFirstPacket = true
				// send ack for the first packet soon to probe the rtt
				sess.sendAck()
			}
		case <-sendTimer.C:
			//fmt.Printf("sendTimer fired\n")
			//sess.sendData()
			ct := utils.NewCostTimer()
			sess.sendPackets()
			if sess.finSent && sess.closeRemote && !closeSet {
				sess.logger.Debug("%s ready to exit", sess)
				if !closeTimer.Stop() {
					<-closeTimer.C
				}
				closeTimer.Reset(time.Second * 10)
				closeSet = true
			}
			sendTimer.Reset(packet.SessionSendInterval)
			//if packet.SessionSendInterval*2 > sess.GetRtt() && sess.GetRtt() != 0 {
			//	sendTimer.Reset(sess.GetRtt() / 2)
			//} else {
			//	sendTimer.Reset(packet.SessionSendInterval)
			//}
			sess.logger.Debug("sendPackets cost %s, rtt %s", ct.Cost(), sess.GetRtt())
		case <-closeTimer.C:
			break loop
		}
	}
	close(sess.sentPackets)
	sess.logger.Info("%s exit main cycle", sess)
}

func (sess *Session) createAckFrame() *packet.AckFrame {
	maxRangeCount := packet.MaxAckFrameDataSize / 16
	if maxRangeCount > packet.MaxAckFrameRangeCount {
		maxRangeCount = packet.MaxAckFrameRangeCount
	}
	ranges := sess.ackManager.queueAckRanges(maxRangeCount)
	if len(ranges) == 0 {
		return nil
	}
	ackFrame := packet.CreateAckFrame(ranges)
	sess.logger.Debug("sending ack frame %s", ackFrame)
	return ackFrame
}

func (sess *Session) sendAck() {
	pkt := packet.NewPacket(sess.conv, sess.nextPktSeq, 0)
	ackFrame := sess.createAckFrame()
	if ackFrame == nil {

	}
	pkt.AddFrame(ackFrame)
	err := sess.sendPacket(pkt)
	if err != nil {
		sess.logger.Error("send packet error: %v", err)
	}
}

func (sess *Session) sendPackets() {
	// how many data can we send
	cwd := sess.congestion.GetCongestionWindow()
	inflight := sess.history.Inflight()
	var canSend int64
	canSend = cwd - inflight
	packets := make([]internal.PacketInfo, 0)

	// send retransmission first
	// timeout packets
	rtoPkts := sess.history.FindTimeoutPacket()
	sess.retransmissionQueue = append(sess.retransmissionQueue, internal.ExtractPktFromItems(rtoPkts)...)
	sess.congestion.OnPacketsLost(internal.PacketsToInfo(rtoPkts))
	sess.logger.Debug("%s find rto packets %v", sess, internal.LogPacketSeq(rtoPkts))
	// fast retransmit packets
	fastRetransPkts := sess.history.FindFastRetransmitPacket()
	sess.retransmissionQueue = append(sess.retransmissionQueue, internal.ExtractPktFromItems(fastRetransPkts)...)
	sess.congestion.OnPacketsLost(internal.PacketsToInfo(fastRetransPkts))
	sess.logger.Debug("%s find fast retransmit packets %v", sess, internal.LogPacketSeq(fastRetransPkts))
	sess.congestion.UpdatePacingThreshold()

	retransNum := 0
	newNum := 0
	ackNum := 0
	canPacing := true
	for canSend > 0 && canPacing && len(sess.retransmissionQueue) > 0 {
		rtoPkt := sess.retransmissionQueue[0]
		sess.retransmissionQueue = sess.retransmissionQueue[1:]
		if !sess.history.IsInflight(rtoPkt.Seq) {
			continue
		}
		retrans := packet.NewPacket(sess.conv, sess.nextPktSeq, 0)
		for _, frame := range rtoPkt.Frames {
			if frame.IsRetransmittable() {
				retrans.AddFrame(frame)
			}
		}
		if len(retrans.Frames) == 0 {
			continue
		}
		err := sess.sendRetransmitPacket(retrans, rtoPkt.Seq)
		if err != nil {
			sess.logger.Error("send packet error: %v", err)
		}
		packets = append(packets, internal.PacketInfo{
			Seq:   retrans.Seq,
			Size:  retrans.Size(),
			Round: sess.history.CurrentRound(),
		})
		canSend -= int64(retrans.Size())
		canPacing = sess.congestion.PacingSend(int64(retrans.Size()))
		retransNum++
	}
	// then send new data
	createCost := time.Duration(0)
	sendCost := time.Duration(0)
	ct := utils.NewCostTimer()
	for canSend > 0 && canPacing && sess.haveDataToSend() {
		ct.Reset()
		pkt := sess.createPacket(int(canSend))
		createCost += ct.Cost()
		if len(pkt.Frames) == 0 {
			break
		}
		pktInfo := internal.PacketInfo{
			Seq:   pkt.Seq,
			Size:  pkt.Size(),
			Round: sess.history.CurrentRound(),
		}
		ct.Reset()
		packets = append(packets, pktInfo)
		err := sess.sendPacket(pkt)
		sendCost += ct.Cost()
		if err != nil {
			sess.logger.Error("send packet error: %v", err)
		}
		canSend -= int64(pkt.Size())
		canPacing = sess.congestion.PacingSend(int64(pkt.Size()))
		newNum++
	}
	sess.logger.Debug("create cost %s, send cost %s", createCost, sendCost)
	// finally see whether need send ack only packet
	if sess.needAck {
		pkt := sess.createAckOnlyPacket()
		if len(pkt.Frames) != 0 {
			pktInfo := internal.PacketInfo{
				Seq:   pkt.Seq,
				Size:  pkt.Size(),
				Round: sess.history.CurrentRound(),
			}
			packets = append(packets, pktInfo)
			err := sess.sendPacket(pkt)
			if err != nil {
				sess.logger.Error("send packet error: %v", err)
			}
		}
		ackNum++
	}
	sess.history.UpdateLowestTracked()
	if len(packets) != 0 {
		sess.congestion.OnPacketsSend(packets)
		sess.logger.Debug("sendPackets done, total: %d, cwd: %d, inFlight: %d, limitedPacing: %v, retransNum: %d, newNum: %d, ackNum: %d", len(packets),
			sess.congestion.GetCongestionWindow(),
			sess.history.Inflight(),
			!canPacing,
			retransNum,
			newNum,
			ackNum)
	}

}

func (sess *Session) createPacket(maxDataSize int) packet.Packet {
	pkt := packet.NewPacket(sess.conv, sess.nextPktSeq, 0)
	remainPayload := packet.Ipv4PayloadSize
	if sess.needAck {
		sess.needAck = false
		ackFrame := sess.createAckFrame()
		if ackFrame != nil {
			pkt.AddFrame(ackFrame)
			remainPayload -= ackFrame.Size()
		}
	}
	size := maxDataSize
	if size > remainPayload-packet.DataFrameHeaderSize {
		size = remainPayload - packet.DataFrameHeaderSize
	}
	if size > 0 || sess.closeLocal {
		dataFrame, _ := sess.popDataFrame(size)
		if dataFrame != nil {
			pkt.AddFrame(dataFrame)
		}
	}

	return pkt
}

func (sess *Session) createAckOnlyPacket() packet.Packet {
	pkt := packet.NewPacket(sess.conv, sess.nextPktSeq, 0)
	if sess.needAck {
		sess.needAck = false
		ackFrame := sess.createAckFrame()
		if ackFrame != nil {
			pkt.AddFrame(ackFrame)
		}
	}
	return pkt
}

func (sess *Session) handlePacket(packet *packet.Packet) {
	sess.logger.Debug("<--receive packet %s", packet)
	sess.ackManager.addAckRange(packet.Seq, packet.Seq)
	//fmt.Println(sess.ackManager.printAckRanges())
	for _, frame := range packet.Frames {
		sess.handleFrame(frame)
	}
	if packet.IsRetransmittable() {
		sess.needAck = true
	}
}

func (sess *Session) handleFrame(frame packet.Frame) error {
	switch frame.Command() {
	case packet.DataFrameCommand:
		dataFrame, ok := frame.(*packet.DataFrame)
		if !ok {
			return fmt.Errorf("convert Frame to DataFrame error")
		}
		sess.dataManager.AddDataRange(dataFrame.Offset, dataFrame.Offset+uint64(len(dataFrame.Data))-1, dataFrame.Data, dataFrame.Fin)
		if dataFrame.Fin == true {
			sess.closeRemote = true
		}
		sess.SignalRead()
	//fmt.Printf("printint data ranges : %s\n", sess.dataManager.PrintDataRanges())
	case packet.AckFrameCommand:
		ackFrame, ok := frame.(*packet.AckFrame)
		if !ok {
			return fmt.Errorf("convert Frame to AckFrame error")
		}
		if ackFrame.RangeCount == 0 || len(ackFrame.AckRanges) == 0 {
			sess.logger.Notice("droped ackFrame for have no range")
			break
		}
		//fmt.Printf("receive ackFrame: %s\n", ackFrame)
		ackInfos := sess.history.AckPackets(ackFrame.AckRanges)
		var minRtt time.Duration
		for _, ackinfo := range ackInfos {
			if minRtt == 0 || minRtt > ackinfo.Rtt {
				minRtt = ackinfo.Rtt
			}
			sess.ackPacket(ackinfo.Seq, ackinfo.Rtt)
		}
		sess.congestion.OnPacketsAck(ackInfos)
		sess.rttStat.Update(minRtt)
		sess.congestion.UpdateRtt(sess.GetRtt())
		sess.history.UpdateLowestTracked()
		sess.logger.Debug("lastedtRtt: %s, smoothRtt: %s, rto: %s\n", minRtt, sess.rttStat.SmoothRtt(), sess.rttStat.Rto())

	}
	return nil
}

func (sess *Session) SignalWrite() {
	select {
	case sess.writeSignal <- struct{}{}:
	default:
	}
}

func (sess *Session) SignalData() {
	select {
	case sess.dataSignal <- struct{}{}:
	default:
	}
}

func (sess *Session) SignalRead() {
	select {
	case sess.readSignal <- struct{}{}:
	default:
	}
}

func (sess *Session) SignalAck() {
	select {
	case sess.ackChan <- struct{}{}:
	default:
	}
}

func (sess *Session) SignalClose() {
	select {
	case sess.closeSignal <- struct{}{}:
	default:
	}
}

func (sess *Session) haveDataToSend() bool {
	sess.sessMux.RLock()
	defer sess.sessMux.RUnlock()
	return sess.writeBufferLen > 0 || (sess.closeLocal && !sess.finSent)
}

func (sess *Session) popDataFrame(size int) (*packet.DataFrame, int) {
	dataSize := size
	sess.sessMux.Lock()
	if dataSize > sess.writeBufferLen {
		dataSize = sess.writeBufferLen
	}
	data := make([]byte, dataSize)
	copy(data, sess.writeBuffer[0:dataSize])
	copy(sess.writeBuffer[0:], sess.writeBuffer[dataSize:])
	sess.writeBufferLen -= dataSize
	remain := sess.writeBufferLen
	sess.SignalWrite()
	sess.sessMux.Unlock()
	fin := false
	if sess.closeLocal && sess.writeBufferLen == 0 {
		fin = true
		sess.finSent = true
	}
	dataFrame := packet.CreateDataFrame(data, sess.nextDataOffset, fin)
	sess.nextDataOffset += uint64(dataSize)
	sess.logger.Debug("popDataFrame Size: %d, dataSize: %d, nextDataOffset: %d, remain: %d, fin: %v", size, dataSize, sess.nextDataOffset, remain, fin)
	return dataFrame, remain
}

func (sess *Session) sendPacket(p packet.Packet) error {
	sess.history.SendPacket(p, sess.rttStat.Rto())
	sess.nextPktSeq++
	sess.logger.Debug("--> %s send packet %s", sess, p)
	if rand.Uint32()%100 < uint32(sess.lossRate) {
		sess.logger.Debug("%s lost packet Seq: %d", sess, p.Seq)
		return nil
	}
	select {
	case sess.sentPackets <- &p:
	default:
		sess.logger.Debug("%s sentPackets is full", sess)
	}

	return nil
}

func (sess *Session) sendRetransmitPacket(p packet.Packet, retransmitFor uint64) error {
	sess.history.SendRetransmitPacket(p, sess.rttStat.Rto(), retransmitFor)
	sess.nextPktSeq++
	sess.logger.Debug("%s send retransmit packet Seq: %d, Size: %d, retransmit for %d", sess, p.Seq, p.Size(), retransmitFor)
	select {
	case sess.sentPackets <- &p:
	default:
		sess.logger.Info("%s sentPackets is full", sess)
	}
	return nil
}

func (sess *Session) ackPacket(seq uint64, rtt time.Duration) {
	sess.logger.Debug("Acked new packet [%d], Rtt: %s, inflight: %d", seq, rtt, sess.history.Inflight())
}

func (sess *Session) SetLossRate(loss int) {
	sess.lossRate = loss
}

func (sess *Session) GetRtt() time.Duration {
	return sess.rttStat.SmoothRtt()
}

type ackManager struct {
	rangeList *list.List
}

func newAckManager() *ackManager {
	return &ackManager{
		rangeList: list.New(),
	}
}

func (manager *ackManager) printAckRanges() string {
	buffer := bytes.Buffer{}
	for cur := manager.rangeList.Front(); cur != nil; cur = cur.Next() {
		buffer.WriteString(fmt.Sprintf("%s ", cur.Value.(packet.AckRange)))
	}
	return buffer.String()
}

func (manager *ackManager) queueAckRanges(maxCount int) []packet.AckRange {
	last := manager.rangeList.Back()
	ranges := make([]packet.AckRange, 0, maxCount)
	for last != nil {
		if len(ranges) >= maxCount {
			break
		}
		ranges = append(ranges, last.Value.(packet.AckRange))
		last = last.Prev()
	}
	return ranges
}

func (manager *ackManager) addAckRange(left, right uint64) {
	if manager.rangeList.Back() == nil {
		ran := packet.AckRange{
			Left:  left,
			Right: right,
		}
		manager.rangeList.PushBack(ran)
		return
	}
	head := manager.rangeList.Front()
	currentRange := packet.AckRange{
		Left:  left,
		Right: right,
	}
	startMerge := false
	cur := head
	ccur := head
	for ; cur != nil; cur = ccur {
		ccur = cur.Next()
		if !CanMerge(cur.Value.(packet.AckRange), currentRange) {
			if !startMerge {
				continue
			} else {
				manager.rangeList.InsertBefore(currentRange, cur)
				//fmt.Printf("insert range %s\n", currentRange)
				break
			}
		}
		currentRange = Merge(currentRange, cur.Value.(packet.AckRange))
		manager.rangeList.Remove(cur)
		//fmt.Printf("removed range %s\n", cur.Value.(AckRange))
		startMerge = true
	}
	if cur == nil {
		manager.rangeList.PushBack(currentRange)
	}
}

func CanMerge(range1, range2 packet.AckRange) bool {
	if range1.Left < range2.Left-1 && range1.Right < range2.Left-1 && range2.Left != 0 {
		// range1在range2的左侧
		return false
	} else if range2.Left < range1.Left-1 && range2.Right < range1.Left-1 && range1.Left != 0 {
		// range2在range1的左侧
		return false
	}
	return true
}

func Merge(range1, range2 packet.AckRange) packet.AckRange {
	merged := packet.AckRange{}
	merged.Left = min(range1.Left, range2.Left)
	merged.Right = max(range1.Right, range2.Right)
	return merged
}

func max(a, b uint64) uint64 {
	if a > b {
		return a
	} else {
		return b
	}
}

func min(a, b uint64) uint64 {
	if a < b {
		return a
	} else {
		return b
	}
}
