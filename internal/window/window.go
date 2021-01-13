package window

import (
	"context"
	"sync/atomic"
	"time"
)

// package window: 提供环形窗口统计

// Index: 指标信息
type Index struct {
	Name  string
	Score uint
}

type Conf struct {
	// size: 窗口大小
	size uint

	// interval: 时间间隔
	interval time.Duration

	ctx context.Context
}

// Window: 窗口对象
type Window struct {

	// Conf: 配置信息
	Conf

	// index: 指针指向的位置
	index uint

	// cancel: 关闭函数
	cancel context.CancelFunc

	// close: 是否关闭
	close uint32

	// communication: 通讯 channel 将kv数据发送到临界区
	communication chan Index

	// buffer: 根据窗口大小生成的buffer数组
	// map[string]uint
	buffer []atomic.Value
}

// Sentinel: 初始化window对象后 后台开始滚动计数并同步更新到total
func (w *Window) Sentinel() {
	tick := time.Tick(w.interval)
	m := make(map[string]uint)
	for {
		select {
		case info, ok := <-w.communication:
			if !ok {
				return
			}
			m[info.Name] += info.Score
		case _, ok := <-tick:
			if !ok {
				// 退出
				return
			}
			index := (w.index + 1) % w.size
			w.buffer[w.index].Store(m)
			m = make(map[string]uint)
			// 最后在赋值
			w.index = index
		case <-w.ctx.Done():
			return
		}
	}
}

// TemporaryBuffer: 临界区buffer 收集kv在时钟信号到来的时候上载到 buffer 中
func (w *Window) TemporaryBuffer() {
	m := make(map[string]uint)
	for info := range w.communication {
		m[info.Name] += info.Score
	}
}

// Shutdown: 关闭
func (w *Window) Shutdown() {
	if atomic.SwapUint32(&w.close, 1) == 1 {
		// 已经执行过close了
		return
	}
	w.cancel()
}

// AddIndex: 添加指标
func (w *Window) AddIndex(k string, v uint) {
	w.communication <- Index{
		Name:  k,
		Score: v,
	}
}

// Show: 展示total
func (w *Window) Show() []Index {
	res := make([]Index, 0)
	m := make(map[string]uint)
	for _, v := range w.buffer {
		buf := v.Load().(map[string]uint)
		for s, u := range buf {
			m[s] += u
		}
	}
	for s, u := range m {
		res = append(res, Index{
			Name:  s,
			Score: u,
		})
	}
	return res
}