package YCP

import (
	"bytes"
	"code.int-2.me/yuyyi51/YCP/internal"
	"code.int-2.me/yuyyi51/YCP/packet"
	"code.int-2.me/yuyyi51/YCP/utils"
	"container/list"
	"fmt"
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

	congestion CongestionAlgorithm

	nextPktSeq     uint64
	nextDataOffset uint64

	history *internal.PacketHistory

	needAck            bool
	receiveFirstPacket bool

	rttStat internal.RttStat

	logger *utils.Logger
}

func NewSession(conn net.PacketConn, addr net.Addr, conv uint32, logger *utils.Logger) *Session {
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
		congestion:      NewRenoAlgorithm(),
		readingMux:      new(sync.RWMutex),
		history:         internal.NewPacketHistory(),
		ackChan:         make(chan struct{}, 1),
		logger:          logger,
	}
	go session.run()
	return session
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
		data := sess.dataManager.PopData()
		sess.logger.Debug("pop data, len %d\n%s", len(data), data)
		if remain >= len(data) {
			copy(b[offset:], data)
			offset += len(data)
		} else {
			copy(b[offset:], data[:remain])
			sess.dataBuffer = data[remain:]
			offset += remain
			break
		}
		if offset == 0 {
			// no data
			select {
			case <-sess.readSignal:
			}
		} else {
			break
		}
	}
	return offset, nil
}

func (sess *Session) Write(b []byte) (n int, err error) {
	sess.logger.Debug("start Write, len %d", len(b))
	sess.writingMux.Lock()
	defer sess.writingMux.Unlock()

	sess.sessMux.Lock()
	if len(b)+sess.writeBufferLen <= len(sess.writeBuffer) {
		// enough space in buffer
		copy(sess.writeBuffer[sess.writeBufferLen:], b)
		sess.writeBufferLen += len(b)
		sess.SignalData()
		sess.sessMux.Unlock()
		sess.logger.Debug("finish Write")
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
			//sess.SignalData()
		}
		if n >= len(b) {
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
	sess.sessMux.Unlock()
	sess.logger.Debug("finish Write")
	return
}

func (sess *Session) Close() error {
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
	retransmissionTimer := time.NewTimer(packet.SessionInitRto)
	for {
		select {
		case pkt := <-sess.receivedPackets:
			sess.handlePacket(pkt)
			if !sess.receiveFirstPacket {
				sess.receiveFirstPacket = true
				// send ack for the first packet soon to probe the rtt
				sess.sendAck()
			}
		//case <-sess.dataSignal:
		//	//fmt.Printf("dataSignal fired\n")
		//	sess.sendData()
		//	sendTimer.Reset(packet.SessionSendInterval)
		case <-sendTimer.C:
			//fmt.Printf("sendTimer fired\n")
			sess.sendData()
			sendTimer.Reset(packet.SessionSendInterval)
		case <-sess.closeSignal:
		case <-retransmissionTimer.C:
			sess.sendRetransmission()
			retransmissionTimer.Reset(packet.SessionInitRto)
		}
	}
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

func (sess *Session) sendRetransmission() {
	pkts := sess.history.FindTimeoutPacket()
	infos := make([]PacketInfo, 0)
	for _, pkt := range pkts {
		retrans := packet.NewPacket(sess.conv, sess.nextPktSeq, 0)
		for _, frame := range pkt.Frames {
			if frame.IsRetransmittable() {
				retrans.AddFrame(frame)
			}
		}
		if len(retrans.Frames) == 0 {
			continue
		}
		sess.history.SendRetransmitPacket(retrans)
		sess.sendRetransmitPacket(retrans, pkt.Seq)
		infos = append(infos, PacketInfo{
			seq:  pkt.Seq,
			size: pkt.Size(),
		})
	}
	//sess.congestion.OnPacketsLost(infos)
	//sess.congestion.OnPacketsSend(infos)
}

func (sess *Session) sendData() {
	// 能发送多少数据
	cwd := sess.congestion.GetCongestionWindow()
	inflight := sess.history.Inflight()
	var canSend uint64
	if cwd > inflight {
		canSend = cwd - inflight
	}

	packets := make([]PacketInfo, 0)

	for {
		var pkt packet.Packet
		if canSend > 0 && sess.haveDataToSend() {
			ct := utils.NewCostTimer()
			pkt = sess.createPacket(int(canSend))
			sess.logger.Debug("createPacket cost %s", ct.Cost())
		} else if sess.needAck {
			pkt = sess.createAckOnlyPacket()
		} else {
			break
		}
		if len(pkt.Frames) == 0 {
			break
		}
		pktInfo := PacketInfo{
			seq:  pkt.Seq,
			size: pkt.Size(),
		}
		packets = append(packets, pktInfo)
		err := sess.sendPacket(pkt)
		if err != nil {
			sess.logger.Error("send packet error: %v", err)
		}
	}
	if len(packets) == 0 {
		// send nothing
		return
	}
	sess.congestion.OnPacketsSend(packets)
	sess.logger.Debug("cwd: %d, inFlight: %d", sess.congestion.GetCongestionWindow(), sess.history.Inflight())
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
	if size > 0 {
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
	sess.logger.Trace("<--receive packet %s", packet)
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
		if len(dataFrame.Data) == 0 {
			sess.logger.Notice("droped dataFrame for have no data")
			break
		}
		sess.dataManager.AddDataRange(dataFrame.Offset, dataFrame.Offset+uint64(len(dataFrame.Data))-1, dataFrame.Data)
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
		sess.rttStat.Update(minRtt)
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

func (sess *Session) haveDataToSend() bool {
	sess.sessMux.Lock()
	defer sess.sessMux.Unlock()
	return sess.writeBufferLen > 0
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
	dataFrame := packet.CreateDataFrame(data, sess.nextDataOffset)
	sess.nextDataOffset += uint64(dataSize)
	return dataFrame, remain
}

func (sess *Session) sendPacket(p packet.Packet) error {
	sess.history.SendPacket(p)
	sess.nextPktSeq++
	sess.logger.Trace("--> %s send packet %s", sess, p)
	rand.Seed(time.Now().UnixNano())
	if rand.Uint64()%100 < 0 {
		sess.logger.Debug("%s lost packet seq: %d", sess, p.Seq)
		return nil
	}
	_, err := sess.conn.WriteTo(p.Pack(), sess.remoteAddr)
	return err
}

func (sess *Session) sendRetransmitPacket(p packet.Packet, retransmitFor uint64) error {
	sess.history.SendRetransmitPacket(p)
	sess.nextPktSeq++
	sess.logger.Debug("%s send retransmit packet seq: %d, size: %d, retransmit for %d", sess, p.Seq, p.Size(), retransmitFor)
	_, err := sess.conn.WriteTo(p.Pack(), sess.remoteAddr)
	return err
}

func (sess *Session) ackPacket(seq uint64, rtt time.Duration) {
	sess.logger.Debug("Acked new packet [%d], Rtt: %s", seq, rtt)

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
