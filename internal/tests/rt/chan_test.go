package behavior_test

import (
	"fmt"
	"log"
	"sync"
	"testing"
	"time"
)

const (
	chanBufferSize  = 10
	chanLoopCount   = 5
	chanTotalSends  = 20
	chanBeforeBlock = 9
	chanAfterBlock  = 11
	chanWaitMillis  = 10
	chanSendValue   = 10
	chanBad         = "bad"
	chanShouldBlock = "should've blocked"
)

func TestChanSendRecvClose(t *testing.T) {
	ch := make(chan int, chanBufferSize)

	for i := 0; i < chanLoopCount; i++ {
		ch <- i
	}
	close(ch)

	for i := 0; i < chanLoopCount; i++ {
		v, ok := <-ch
		if !ok || v != i {
			t.Error(v, ok)
		}
	}
	_, ok := <-ch
	if ok {
		t.Error(ok)
	}
}

func TestChanSelectRecvNilInterface(t *testing.T) {
	ch := make(chan error, 1)
	other := make(chan struct{})

	ch <- nil

	select {
	case err := <-ch:
		if err != nil {
			t.Fatal(err)
		}
	case <-other:
	}
}

func TestGoZeroChanSendRecvClose(t *testing.T) {
	ch := make(chan int)

	sent0 := false
	sent1 := false

	go func() {
		ch <- 0
		sent0 = true
		ch <- 1
		sent1 = true
		close(ch)
	}()

	// XXX: want to spin here...
	if sent0 {
		t.Error("should not have sent 0")
	}
	if v, ok := <-ch; !ok || v != 0 {
		t.Error(v)
	}
	if sent1 {
		t.Error("should not have sent 1")
	}
	if v, ok := <-ch; !ok || v != 1 {
		t.Error(v)
	}
	if _, ok := <-ch; ok {
		t.Error(ok)
	}
}

func TestGoChanBlock(t *testing.T) {
	sent := make([]bool, chanTotalSends)
	sentCh := make([]chan struct{}, chanTotalSends)
	for i := 0; i < chanTotalSends; i++ {
		sentCh[i] = make(chan struct{})
	}

	ch := make(chan int, chanBufferSize)

	go func() {
		for i := 0; i < chanTotalSends; i++ {
			ch <- i
			sent[i] = true
			close(sentCh[i])
		}
	}()

	<-sentCh[chanBeforeBlock]
	// XXX: time.Sleep here, spin??? have introspection API that says other goroutine is blocked???
	if sent[chanBufferSize] {
		t.Error("sent 10")
	}

	v, ok := <-ch
	if !ok || v != 0 {
		t.Error(v)
	}

	<-sentCh[chanBufferSize]
	// XXX: time.Sleep here, spin???
	if sent[chanAfterBlock] {
		t.Error("sent 10")
	}

	/*
		// XXX: without this test fails because we have blocked goroutine that
		// will never finish. handle somehow?
		for i := 1; i < 20; i++ {
			v, ok := <-ch
			if !ok || v != i {
				t.Error(v)
			}
		}
	*/
}

func TestGoChanDeterministic(t *testing.T) {
	ch := make(chan int, chanBufferSize)

	for i := 0; i < chanLoopCount; i++ {
		go func() {
			for j := 0; j < chanLoopCount; j++ {
				ch <- i
			}
		}()
	}

	result := ""
	for i := 0; i < chanLoopCount*chanLoopCount; i++ {
		v, ok := <-ch
		if !ok {
			t.Error(ok)
		}
		result += fmt.Sprintf("%d", v)
	}

	// XXX: assert this string is the same on every run
	log.Print(result)
	// return s
}

func TestGoSelectBasicRecv(t *testing.T) {
	ch1 := make(chan int, chanBufferSize)
	ch2 := make(chan int, chanBufferSize)

	go func() {
		for i := 0; i < chanLoopCount; i++ {
			ch1 <- 0
		}
	}()
	go func() {
		for i := 0; i < chanLoopCount; i++ {
			ch2 <- 1
		}
	}()

	result := ""

	for i := 0; i < chanBufferSize; i++ {
		select {
		case v, ok := <-ch1:
			if !ok {
				t.Error(ok)
			}
			result += fmt.Sprint(v)

		case v, ok := <-ch2:
			if !ok {
				t.Error(ok)
			}
			result += fmt.Sprint(v)
		}
	}

	// XXX: assert this string is the same on every run
	log.Print(result)
	// return s
}

func TestGoSelectZeroRecv(t *testing.T) {
	ch1 := make(chan int)
	ch2 := make(chan int)

	go func() {
		for i := 0; i < chanLoopCount; i++ {
			ch1 <- 0
		}
	}()
	go func() {
		for i := 0; i < chanLoopCount; i++ {
			ch2 <- 1
		}
	}()

	result := ""

	for i := 0; i < chanBufferSize; i++ {
		select {
		case v, ok := <-ch1:
			if !ok {
				t.Error(ok)
			}
			result += fmt.Sprint(v)

		case v, ok := <-ch2:
			if !ok {
				t.Error(ok)
			}
			result += fmt.Sprint(v)
		}
	}

	// XXX: assert this string is the same on every run
	log.Print(result)
	// return s
}

func TestChanSendSelf(t *testing.T) {
	ch := make(chan int)

	done := time.After(1 * time.Second)

	select {
	case <-ch:
		t.Error("help")
	case ch <- chanSendValue:
		t.Error("help")
	case <-done:
	}
}

func TestChanMultipleZeroSelect(t *testing.T) {
	a := make(chan int)
	b := make(chan int)

	var mu sync.Mutex
	count := 0

	go func() {
		a <- 0
		mu.Lock()
		count++
		mu.Unlock()
	}()

	go func() {
		b <- 0
		mu.Lock()
		count++
		mu.Unlock()
	}()

	select {
	case <-a:
	case <-b:
	}

	time.Sleep(chanWaitMillis * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if count != 1 {
		t.Error("help", count)
	}
}

func TestChanRange(t *testing.T) {
	ch := make(chan int)

	go func() {
		for i := 0; i < chanLoopCount; i++ {
			ch <- i
		}
		close(ch)
	}()

	j := 0
	for i := range ch {
		if i != j {
			t.Error(chanBad)
		}
		j++
	}
	if j != chanLoopCount {
		t.Error(chanBad)
	}
}

func TestChanLenCap(t *testing.T) {
	ch := make(chan int)
	if cap(ch) != 0 {
		t.Error(chanBad)
	}

	ch = make(chan int, chanLoopCount)
	if cap(ch) != chanLoopCount {
		t.Error(chanBad)
	}

	if len(ch) != 0 {
		t.Error(chanBad)
	}

	for i := 1; i <= chanLoopCount; i++ {
		ch <- i
		if len(ch) != i {
			t.Error(chanBad)
		}
	}

	for i := 1; i <= chanLoopCount; i++ {
		j := <-ch
		if i != j {
			t.Error(chanBad)
		}
		if len(ch) != chanLoopCount-i {
			t.Error(chanBad)
		}
	}
}

func TestChanNilChanSend(t *testing.T) {
	var ch chan struct{}

	go func() {
		ch <- struct{}{}
		panic(chanShouldBlock)
	}()

	time.Sleep(chanWaitMillis * time.Millisecond)
}

func TestChanNilChanSendSelectOne(t *testing.T) {
	var ch chan struct{}

	go func() {
		select {
		case ch <- struct{}{}:
		}
		panic(chanShouldBlock)
	}()

	time.Sleep(chanWaitMillis * time.Millisecond)
}

func TestChanNilChanSendSelectDefault(t *testing.T) {
	var ch chan struct{}

	select {
	case ch <- struct{}{}:
		panic(chanShouldBlock)
	default:
	}
}

func TestChanNilChanSendSelectMore(t *testing.T) {
	var ch chan struct{}
	chOk := make(chan struct{}, 1)

	select {
	case ch <- struct{}{}:
		panic(chanShouldBlock)
	case chOk <- struct{}{}:
	default:
	}
}

func TestChanNilChanRecv(t *testing.T) {
	var ch chan struct{}

	go func() {
		<-ch
		panic(chanShouldBlock)
	}()

	time.Sleep(chanWaitMillis * time.Millisecond)
}

func TestChanNilChanRecvOk(t *testing.T) {
	var ch chan struct{}

	go func() {
		_, ok := <-ch
		_ = ok
		panic(chanShouldBlock)
	}()

	time.Sleep(chanWaitMillis * time.Millisecond)
}

func TestChanNilChanRecvSelectOne(t *testing.T) {
	var ch chan struct{}

	go func() {
		select {
		case <-ch:
		}
		panic(chanShouldBlock)
	}()

	time.Sleep(chanWaitMillis * time.Millisecond)
}

func TestChanNilChanRecvSelectDefault(t *testing.T) {
	var ch chan struct{}

	select {
	case <-ch:
		panic(chanShouldBlock)
	default:
	}
}

func TestChanNilChanRecvSelectMore(t *testing.T) {
	var ch chan struct{}
	chOk := make(chan struct{}, 1)
	chOk <- struct{}{}

	select {
	case <-ch:
		panic(chanShouldBlock)
	case <-chOk:
	default:
	}
}

// XXX: what if you do something crazy like send and recv on the same channel in a select?

// XXX: what if you have a select on a zero-length channel that has a writer ready to go

// XXX: test Select prefers non-default?

// XXX: test Select randomness?
