package main

import (
	"fmt"
	"time"
)

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
	}
}

func main() {
	evt := EvtCreate()

	evt.on("evt", func(data interface{}) {
		fmt.Print(data)
	})
	evt.emit("evt", "aab")
	evt.emit("evt", "aab")
	go func() {
		time.Sleep(time.Millisecond * 400)
		evt.close("evt")
	}()
	go func() {
		time.Sleep(time.Millisecond * 500)
		evt.emit("evt", 1234)
	}()
	evt.setTime(func() {
		fmt.Println("xxxx")
	}, 1000)

	evt.loop()
}
