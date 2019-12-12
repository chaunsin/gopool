package gopool_test

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"testing"
	"time"
)

func TestOne(t *testing.T) {
	//ctx, cancel := context.WithTimeout(context.Background(), time.Minute*3)
	//defer cancel()

	pool := NewContextPool(nil, Queue(1000), Worker(6))
	defer pool.Close()

	for i := 0; i < 1000; i++ {
		ii := i
		//减少
		if i == 100 {
			pool.Resize(-4)
		}
		//增加
		if i == 150 {
			pool.Resize(8)
		}

		//关闭
		if i == 250 {
			pool.Close()
		}

		//关闭后重置数量
		if i == 260 {
			//关闭之后重新设置resize
			pool.Resize(66)
		}

		if i == 261 {
			//关闭之后添加
			pool.SyncAdd(func() error {
				time.Sleep(time.Millisecond * time.Duration(rand.Int31n(1000)))
				fmt.Println("第", ii)
				return nil
			})
			continue
		}
		if i == 263 {
			//关闭之后添加
			pool.Add(func() error {
				time.Sleep(time.Millisecond * time.Duration(rand.Int31n(1000)))
				fmt.Println("第", ii)
				return nil
			})
			continue
		}

		pool.Add(func() error {
			time.Sleep(time.Millisecond * time.Duration(rand.Int31n(1000)))
			fmt.Println("第", ii)
			return nil
		})
	}
}

func TestTwo(t *testing.T) {
	pool := NewContextPool(nil, Queue(100), Worker(4))
	for i := 0; i < 20; i++ {
		ii := i
		go func() {
			if err := pool.SyncAdd(func() error {
				time.Sleep(time.Millisecond * time.Duration(rand.Int31n(1000)))
				fmt.Printf("[%d]\n", ii)
				return nil
			}); err != nil {
				log.Println("[sync]", ii, err)
			}
		}()
	}

	pool.Close()

	pool.Resize(2)

	for i := 0; i < 10; i++ {
		ii := i
		go func() {
			if err := pool.SyncAdd(func() error {
				time.Sleep(time.Millisecond * time.Duration(rand.Int31n(1000)))
				fmt.Printf("[%d]\n", ii)
				return nil
			}); err != nil {
				log.Println("[sync]", ii, err)
			}
			if err := pool.Add(func() error {
				time.Sleep(time.Millisecond * time.Duration(rand.Int31n(1000)))
				fmt.Println("one")
				return nil
			}); err != nil {
				log.Println("[add]", err)
			}
		}()
	}

	time.Sleep(time.Second * 5)

}

func TestThree(t *testing.T) {
	var (
		wg   sync.WaitGroup
		pool     = NewContextPool(nil, Queue(5000), Worker(10))
		size int = 0
		//lock sync.Mutex
	)
	defer pool.Close()

	wg.Add(5000)
	for i := 0; i < 5000; i++ {
		ii := i
		go func() {
			defer wg.Done()
			if ii%200 == 0 {
				//lock.Lock()
				//size -= 35
				//pool.Resize(size)
				//size += 45
				//size += 1
				_ = size
				pool.Resize(2)
				//lock.Unlock()
			}
			if err := pool.SyncAdd(func() error {
				time.Sleep(time.Millisecond * time.Duration(rand.Int31n(1000)))
				fmt.Printf("[%d]\n", ii)
				return nil
			}); err != nil {
				log.Println("[sync]", ii, err)
			}
		}()
	}
	wg.Wait()
}
