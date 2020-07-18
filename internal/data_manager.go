package internal

import (
	"bytes"
	"code.int-2.me/yuyyi51/YCP/utils"
	"container/list"
	"fmt"
	"sync"
)

type DataManager struct {
	rangeList *list.List
	minOffset uint64
	logger    *utils.Logger
	dataMux   *sync.RWMutex
}

type dataRange struct {
	left  uint64
	right uint64
	data  []byte
}

func (ran dataRange) String() string {
	return fmt.Sprintf("[%d,%d|%d]", ran.left, ran.right, len(ran.data))
}

func NewDataManager(logger *utils.Logger) *DataManager {
	return &DataManager{
		rangeList: list.New(),
		logger:    logger,
		dataMux:   new(sync.RWMutex),
	}
}
func (manager *DataManager) PrintDataRanges() string {
	manager.dataMux.RLock()
	defer manager.dataMux.RUnlock()
	buffer := bytes.Buffer{}
	for cur := manager.rangeList.Front(); cur != nil; cur = cur.Next() {
		buffer.WriteString(fmt.Sprintf("%s ", cur.Value.(dataRange)))
	}
	return buffer.String()
}

func (manager *DataManager) PopData() []byte {
	manager.dataMux.RLock()
	defer manager.dataMux.RUnlock()
	buffer := bytes.Buffer{}
	cur := manager.rangeList.Front()
	curr := cur
	for ; cur != nil; cur = curr {
		curr = cur.Next()
		curRange := cur.Value.(dataRange)
		if curRange.left == manager.minOffset {
			buffer.Write(curRange.data)
			manager.logger.Debug("pop range %s\n", curRange)
			//fmt.Printf("pop range %s, data %s\n", curRange, hex.EncodeToString(curRange.data))
			manager.minOffset = curRange.right + 1
			manager.rangeList.Remove(cur)
			//fmt.Printf("remove newRange %s\n", cur.Value.(dataRange))
		}
	}
	manager.logger.Debug("new min offset: %d", manager.minOffset)
	//fmt.Printf("new min offset: %d\n", manager.minOffset)
	return buffer.Bytes()
}

func (manager *DataManager) AddDataRange(left, right uint64, data []byte) {
	manager.dataMux.Lock()
	defer manager.dataMux.Unlock()
	if manager.minOffset > right {
		return
	}
	currentRange := dataRange{
		left:  left,
		right: right,
		data:  make([]byte, len(data)),
	}
	copy(currentRange.data, data)
	if manager.rangeList.Back() == nil {
		manager.rangeList.PushBack(currentRange)
		manager.logger.Trace("insert newRange1 %s, \n%s", currentRange, currentRange.data)
		return
	}
	head := manager.rangeList.Front()
	cur := head
	ccur := head
	for ; cur != nil; cur = ccur {
		ccur = cur.Next()
		curRange := cur.Value.(dataRange)
		if haveOverlap(curRange, currentRange) {
			if currentRange.left < curRange.left {
				newRange := dataRange{
					left:  currentRange.left,
					right: curRange.left - 1,
					data:  currentRange.data[:curRange.left-currentRange.left],
				}
				manager.rangeList.InsertBefore(newRange, cur)
				manager.logger.Trace("insert newRange2 %s, \n%s", newRange, newRange.data)
				//fmt.Printf("insert newRange2 %s, %s\n", newRange, hex.EncodeToString(newRange.data))
				break
			} else if currentRange.right > curRange.right {
				newRange := dataRange{
					left:  curRange.right + 1,
					right: currentRange.right,
					data:  currentRange.data[curRange.right+1-currentRange.left:],
				}
				currentRange = newRange
			}
		} else if currentRange.right < curRange.left {
			manager.rangeList.InsertBefore(currentRange, cur)
			manager.logger.Trace("insert newRange3 %s, \n%s", currentRange, currentRange.data)
			//fmt.Printf("insert newRange3 %s, %s\n", currentRange, hex.EncodeToString(currentRange.data))
			break
		}
	}
	if cur == nil && manager.rangeList.Back() != nil && !haveOverlap(manager.rangeList.Back().Value.(dataRange), currentRange) {
		manager.rangeList.PushBack(currentRange)
		manager.logger.Trace("insert newRange4 %s, \n%s", currentRange, currentRange.data)
		//fmt.Printf("insert newRange4 %s, %s\n", currentRange, hex.EncodeToString(currentRange.data))
	}
}

func haveOverlap(range1, range2 dataRange) bool {
	if range1.right < range2.left || range1.left > range2.right {
		return false
	}
	return true
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
