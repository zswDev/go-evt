package main

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// goid ,可能并发，需要在函数defer 时，清楚改goid 队列

func Goid() int {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println("panic recover:panic info:%v", err)
		}
	}()

	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	idField := strings.Fields(strings.TrimPrefix(string(buf[:n]), "goroutine "))[0]
	id, err := strconv.Atoi(idField)
	if err != nil {
		panic(fmt.Sprintf("cannot get goroutine id: %v", err))
	}

	return id
}

// TODO,链表实现，监听者链表
type List struct { // 监听链表
	cb  func(interface{})
	ptr *List
}
type tList struct { // 监听链表
	cb  func()
	ptr *tList
}

type Inbox struct {
	data []interface{} // 信件
	li   *List         // 收件箱
}
type EvtGroup struct {
	inboxs   map[int]map[string]*Inbox
	datelist map[int]map[int64]*tList
}

func eGoid(this EvtGroup) int {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println("panic recover:panic info:%v", err)
		}
	}()

	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	idField := strings.Fields(strings.TrimPrefix(string(buf[:n]), "goroutine "))[0]
	id, err := strconv.Atoi(idField)
	if err != nil {
		panic(fmt.Sprintf("cannot get goroutine id: %v", err))
	}

	// 初始化
	if this.datelist[id] == nil {
		this.datelist[id] = make(map[int64]*tList)
	}
	if this.inboxs[id] == nil {
		this.inboxs[id] = make(map[string]*Inbox)
	}

	return id
}

func EvtCreate() *EvtGroup {
	this := new(EvtGroup)
	this.inboxs = make(map[int]map[string]*Inbox)
	this.datelist = make(map[int]map[int64]*tList) // TODO, 最小堆，
	return this
}

func (this EvtGroup) on(str string, callback func(interface{})) {
	gid := eGoid(this)

	var li List
	li.cb = callback

	inbox := this.inboxs[gid][str]
	if inbox == nil {
		this.inboxs[gid][str] = &Inbox{
			data: make([]interface{}, 0),
			li:   &li,
		}
	} else if inbox.li == nil {
		this.inboxs[gid][str].li = &li
	} else {
		if inbox.li.ptr != nil {
			li.ptr = inbox.li.ptr
		}
		this.inboxs[gid][str].li.ptr = &li
	}
}
func (this EvtGroup) once(str string, callback func(interface{})) {
	gid := eGoid(this)

	var li List
	li.cb = func(i interface{}) {
		callback(i)
		delete(this.inboxs[gid], str)
	}

	inbox := this.inboxs[gid][str]
	if inbox == nil {
		this.inboxs[gid][str] = &Inbox{
			data: make([]interface{}, 0),
			li:   &li,
		}
	} else if inbox.li == nil {
		this.inboxs[gid][str].li = &li
	} else {
		if inbox.li.ptr != nil {
			li.ptr = inbox.li.ptr
		}
		this.inboxs[gid][str].li.ptr = &li
	}
}

func (this EvtGroup) emit(str string, data interface{}) {
	gid := eGoid(this)
	inbox := this.inboxs[gid][str]

	if inbox != nil {
		if inbox.data == nil {
			this.inboxs[gid][str].data = []interface{}{data}
		} else {
			this.inboxs[gid][str].data = append(inbox.data, data)
		}
	} else {
		var li Inbox
		li.data = []interface{}{data}
		this.inboxs[gid][str] = &li
	}
}
func (this EvtGroup) close(str string) {
	gid := eGoid(this)
	delete(this.inboxs[gid], str)
}
func (this EvtGroup) setTime(cb func(), tlen int64) {
	gid := eGoid(this)
	tlen = time.Now().UnixNano()/1000000 + tlen
	lis := this.datelist[gid][tlen]
	var tli tList
	tli.cb = cb
	if lis == nil {
		this.datelist[gid][tlen] = &tli
	} else {
		if lis.ptr != nil {
			tli.ptr = lis.ptr
		}
		lis.ptr = &tli
	}
}
func (this EvtGroup) setTimeLoop(cb func(), tlen int64) {
	gid := eGoid(this)
	tkey := time.Now().UnixNano()/1000000 + tlen
	lis := this.datelist[gid][tkey]
	var tli tList

	tli.cb = func() {
		//fmt.Println("aa1")
		this.setTimeLoop(cb, tlen)
		cb()
	}
	if lis == nil {
		this.datelist[gid][tkey] = &tli
	} else {
		if lis.ptr != nil {
			tli.ptr = lis.ptr
		}
		lis.ptr = &tli
	}
}

func dgRun(li *List, data interface{}) {
	if li == nil {
		return
	}
	li.cb(data)
	dgRun(li.ptr, data)
}
func tdgRun(tli *tList) {
	if tli == nil {
		return
	}
	tli.cb()
	tdgRun(tli.ptr)
}

func (this EvtGroup) loop() { // 多协程事件循环
	thisgid := eGoid(this)
	defer func() {
		this.inboxs[thisgid] = nil
		this.datelist[thisgid] = nil
		fmt.Println(thisgid, "delete data")
	}()

	for {
		// 事件发射通知，全部协程都收取
		for gid, nodeMap := range this.inboxs {
			// 获取某个节点的所有收件箱
			for evtname, inbox := range nodeMap {
				emitdata := inbox.data
				oncb := inbox.li
				for _, data := range emitdata {
					if oncb != nil && this.inboxs[gid][evtname] != nil {
						dgRun(oncb, data) // 这里可能会导致 inbox被回收, once
					}
				}
				if this.inboxs[gid][evtname] != nil {
					this.inboxs[gid][evtname].data = []interface{}{}
				}
			}
		}

		// 时间循环
		now := time.Now().UnixNano() / 1000000
		for i, cbs := range this.datelist[thisgid] {
			if i <= now {
				tdgRun(cbs)
				delete(this.datelist[thisgid], i)
			}
		}
		if len(this.datelist[thisgid]) == 0 {
			// 如果无延时了就退出
			ei := false
			for _, inbox := range this.inboxs[thisgid] {
				if len(inbox.data) != 0 {
					ei = true
					break
				}
			}
			if !ei {
				fmt.Println(eGoid(this), ":exit loop")
				return
			}
		}
		time.Sleep(time.Millisecond * 1)
	}
}

func main() {
	evt := EvtCreate()

	defer evt.loop()

	evt.setTime(func() {
		fmt.Println(Goid(), "setTimeout")
		evt.emit("aab", "124")
	}, 1000)
	evt.on("aab", func(i interface{}) {
		fmt.Println("dd", i)
	})
	// 不能 跨越 协程，通信，待完善

	/*evt.setTimeLoop(func() {
		i++
		evt.emit("aab", i)
	}, 1000) */
}
