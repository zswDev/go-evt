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

type EvtGroup struct {
	onMap   map[int]map[string]func(interface{})
	emitMap map[int]map[string][]interface{}
	date    map[int]map[int64][]func()
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
	if this.onMap[id] == nil {
		this.onMap[id] = make(map[string]func(interface{}))
	}
	if this.emitMap[id] == nil {
		this.emitMap[id] = make(map[string][]interface{})
	}
	if this.date[id] == nil {
		this.date[id] = make(map[int64][]func())
	}

	return id
}

// 前一个值是，goid 保证，数据不冲突
func EvtCreate() *EvtGroup {
	this := new(EvtGroup)
	this.onMap = make(map[int]map[string]func(interface{}))
	this.emitMap = make(map[int]map[string][]interface{})
	this.date = make(map[int]map[int64][]func())
	return this
}

func (this EvtGroup) on(str string, callback func(interface{})) {
	gid := eGoid(this)
	this.onMap[gid][str] = callback
}
func (this EvtGroup) once(str string, callback func(interface{})) {
	gid := eGoid(this)
	this.onMap[gid][str] = func(d interface{}) {
		callback(d)
		delete(this.onMap[gid], str)
	}
}
func (this EvtGroup) emit(str string, data interface{}) {
	gid := eGoid(this)
	emit := this.emitMap[gid][str]
	if emit != nil {
		this.emitMap[gid][str] = append(emit, data)
	} else {
		this.emitMap[gid][str] = []interface{}{data}
	}
}
func (this EvtGroup) close(str string) {
	gid := eGoid(this)
	delete(this.onMap[gid], str)
}
func (this EvtGroup) setTime(cb func(), tlen int64) {
	gid := eGoid(this)
	tlen = time.Now().UnixNano()/1000000 + tlen
	cbs := this.date[gid][tlen]
	if cbs != nil {
		this.date[gid][tlen] = append(cbs, cb)
	} else {
		this.date[gid][tlen] = []func(){cb}
	}
}
func (this EvtGroup) loop() { // 多协程事件循环
	thisgid := eGoid(this)
	defer func() {
		this.onMap[thisgid] = nil
		this.emitMap[thisgid] = nil
		this.date[thisgid] = nil
		fmt.Println(thisgid, "delete data")
	}()

	for {
		// 事件通知，全部协程都收取
		for gid, nodeEmitMap := range this.emitMap {
			// 获取某个节点的事件

			for evtname, emitdata := range nodeEmitMap {
				for _, data := range emitdata {
					for _, nodeOnMap := range this.onMap { // 获取某个节点的监听事件
						cb := nodeOnMap[evtname]
						if cb != nil {
							cb(data)
						}
					}
				}
				delete(this.emitMap[gid], evtname)
			}
		}

		// 时间循环, 只在各自协程内调用
		now := time.Now().UnixNano() / 1000000
		for i, cbs := range this.date[thisgid] {
			if i <= now {
				for _, cb := range cbs {
					cb()
				}
				delete(this.date[thisgid], i)
			}
		}
		if len(this.date[thisgid]) == 0 {
			fmt.Println(eGoid(this), ":exit loop")
			return
		}
		time.Sleep(time.Millisecond * 1)
	}
}

/*
单协程通知，可能出错

type Evt struct {
	onMap   map[string]func(interface{})
	emitMap map[string][]interface{}
	date    map[int64][]func()
}
func EvtCreate() *Evt {
	this := new(Evt)
	this.onMap = make(map[string]func(interface{}))
	this.emitMap = make(map[string][]interface{})
	this.date = make(map[int64][]func())
	return this
}
func (this Evt) on(str string, callback func(interface{})) {
	this.onMap[str] = callback
}
func (this Evt) once(str string, callback func(interface{})) {
	this.onMap[str] = func(d interface{}) {
		callback(d)
		delete(this.onMap, str)
	}
}

func (this Evt) emit(str string, data interface{}) {
	fmt.Println(Goid())
	emit := this.emitMap[str]
	if emit != nil {
		this.emitMap[str] = append(emit, data)
	} else {
		this.emitMap[str] = []interface{}{data}
	}
}
func (this Evt) close(str string) {
	delete(this.onMap, str)
}
func (this Evt) setTime(cb func(), tlen int64) {
	tlen = time.Now().UnixNano()/1000000 + tlen
	cbs := this.date[tlen]
	if cbs != nil {
		this.date[tlen] = append(cbs, cb)
	} else {
		this.date[tlen] = []func(){cb}
	}
}
func (this Evt) loop() { // 单线程事件循环

	for {
		time.Sleep(time.Millisecond * 1)
		for evtname, emitdata := range this.emitMap {
			for _, data := range emitdata {
				cb := this.onMap[evtname]
				if cb != nil {
					cb(data)
				}
			}
			delete(this.emitMap, evtname)
		}

		// 时间循环
		now := time.Now().UnixNano() / 1000000
		for i, cbs := range this.date {
			if i <= now {
				for _, cb := range cbs {
					cb()
				}
				delete(this.date, i)
			}
		}
		if len(this.date) == 0 {
			fmt.Println("exit loop")
			return
		}
	}
} */

func main() {
	evt := EvtCreate()
	if evt.emitMap[Goid()] == nil {
		fmt.Println("aaa")
	}
	//evt.emitMap[Goid()] = make(map[string][]interface{})

	evt.on("evt", func(data interface{}) {
		fmt.Println(Goid(), data)
	})
	evt.emit("evt", "aab")
	go func() {
		evt.on("aab", func(data interface{}) {
			fmt.Println(Goid(), data)
		})
		evt.setTime(func() {
			fmt.Println(Goid(), "setTimeout")
		}, 800)
		evt.loop() // 注意事件循环退出
	}()
	go func() {
		//fmt.Println(Goid())
		time.Sleep(time.Millisecond * 500)
		evt.emit("aab", "???")
		evt.emit("evt", 1234)
	}()
	evt.setTime(func() {
		fmt.Println(Goid(), "setTimeout")
	}, 1000)

	evt.loop()
}
