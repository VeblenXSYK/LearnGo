package main

import (
	"context"
	"log"
	"os"
	"time"
)

var logg *log.Logger

func someHandler() {

	// 取消根context(注意：根context不能通过这种方式被取消)
	ctx, cancel := context.WithCancel(context.Background()) // context.Background() 创建根context
	go doStuff(ctx)

	// 取消根context(可以采用这种方式取消根context)
	// WithTimeout 等价于 WithDeadline(parent, time.Now().Add(timeout))
	// ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	// ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(5*time.Second))
	// go doTimeOutStuff(ctx)

	// 10秒后取消ctx
	time.Sleep(10 * time.Second)
	cancel()
}

// 每1秒work一下，同时会判断ctx是否被取消了，如果是就退出
func doStuff(ctx context.Context) {
	for {
		time.Sleep(1 * time.Second)
		select {
		case <-ctx.Done():
			logg.Printf("done")
			return
		default:
			logg.Printf("work")
		}
	}
}

func doTimeOutStuff(ctx context.Context) {
	for {
		time.Sleep(1 * time.Second)

		// 由于之前设置了deadl，此时返回context何时会超时
		if deadline, ok := ctx.Deadline(); ok {
			if time.Now().After(deadline) {
				logg.Printf(ctx.Err().Error())
				return
			}

		}

		select {
		case <-ctx.Done():
			logg.Printf("done")
			return
		default:
			logg.Printf("work")
		}
	}
}

func main() {
	logg = log.New(os.Stdout, "main ", log.Ltime)
	someHandler()
	logg.Printf("down")
}
