package egroup

import (
	"Songzhibin/GKit/goroutine"
	"context"
	"fmt"
	"golang.org/x/sync/errgroup"
	"os"
	"os/signal"
	"syscall"
)

// LifeAdminer: 生命周期管理接口
type LifeAdminer interface {
	Start(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

// Member: 成员
type Member struct {
	Start    func(ctx context.Context) error
	Shutdown func(ctx context.Context) error
}

// LifeAdmin: 生命周期管理
type LifeAdmin struct {
	opts     options
	members  []Member
	shutdown func()
}

// Add: 添加成员表(通过内部 Member 对象添加)
func (l *LifeAdmin) Add(member Member) {
	l.members = append(l.members, member)
}

// AddMember: 添加程序表(通过外部接口 LifeAdminer 添加)
func (l *LifeAdmin) AddMember(la LifeAdminer) {
	l.members = append(l.members, Member{
		Start:    la.Start,
		Shutdown: la.Shutdown,
	})
}

// AddMember: 添加成员表(通过外部接口 LifeAdminer 添加)
func (l *LifeAdmin) Start() error {
	ctx := context.Background()
	ctx, l.shutdown = context.WithCancel(ctx)
	g, ctx := errgroup.WithContext(ctx)
	for _, m := range l.members {
		func(m Member) {
			// 如果有shutdown进行注册
			if m.Shutdown != nil {
				g.Go(func() error {
					// 等待异常或信号关闭触发
					<-ctx.Done()
					return goroutine.Delegate(ctx, l.opts.stopTimeout, m.Shutdown)
				})
			}
			if m.Start != nil {
				g.Go(func() error {
					return goroutine.Delegate(ctx, l.opts.startTimeout, m.Start)
				})
			}
		}(m)
	}
	// 判断是否需要监听信号
	if len(l.opts.signals) == 0 || l.opts.handler == nil {
		return g.Wait()
	}
	c := make(chan os.Signal, len(l.opts.signals))
	// 监听信号
	signal.Notify(c, l.opts.signals...)
	g.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case sig := <-c:
				fmt.Println("捕获到信号")
				l.opts.handler(l, sig)
			}
		}
	})
	return g.Wait()
}

// Shutdown: 停止服务
func (l *LifeAdmin) Shutdown() {
	if l.shutdown != nil {
		l.shutdown()
	}
}

// NewLifeAdmin: 实例化方法
func NewLifeAdmin(opts ...Option) *LifeAdmin {
	// 默认参数
	options := options{
		startTimeout: startTimeout,
		stopTimeout:  stopTimeout,
		signals:      []os.Signal{syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT},
		handler: func(a *LifeAdmin, signal os.Signal) {
			switch signal {
			case syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT:
				a.shutdown()
			default:
			}
		},
	}
	// 选项模式填充参数
	for _, opt := range opts {
		opt(&options)
	}
	return &LifeAdmin{opts: options}
}
