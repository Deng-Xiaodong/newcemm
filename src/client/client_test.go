package client

import (
	"DRW/src/rpc/cemm"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log"
	"strings"
	"sync"
	"testing"
	"time"
)

func getCli(uid int) *EMMClient {
	c, err := grpc.NewClient("127.0.0.1:19091", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	cli := NewEMMClient(uid, cemm.NewCEMMClient(c))
	return cli
}
func TestEMMClient_Init(t *testing.T) {
	cli := getCli(1)
	if err := cli.Init([]string{"w1"}); err != nil {
		log.Fatal(err)
	}
	log.Println("init success")
}
func TestEMMClient_Add(t *testing.T) {
	cli := getCli(2)
	err := cli.Add("w1", "2_3")
	if err != nil {
		t.Fatal(err)
	}
}
func TestEMMClient_Get(t *testing.T) {
	cli := getCli(1)
	if got, err := cli.Get("w1"); err != nil {
		t.Fatal(err)
	} else {
		log.Printf("get success for %s: %v\n", "w1", got)
	}
}

func TestConOp(t *testing.T) {
	work := make(chan string, 1000)
	wg := sync.WaitGroup{}
	//wg.Add(1)
	//go func() {
	//	defer wg.Done()
	//	file, err := os.OpenFile("res.txt", os.O_CREATE|os.O_RDWR, 0777)
	//	if err != nil {
	//		log.Fatal(err)
	//	}
	//	for {
	//		select {
	//		case <-time.After(5 * time.Second):
	//			return
	//		case line := <-work:
	//			file.WriteString(line)
	//		}
	//	}
	//}()

	for i := 1; i <= N; i++ {
		wg.Add(1)
		go func(uid int) {
			defer wg.Done()
			op(uid, work)
		}(i)
	}
	wg.Wait()

}
func op(uid int, work chan string) {
	cli := getCli(uid)
	if uid%2 == 0 {
		for i := 1; i <= 300; i++ {
			err := cli.Add("w1", fmt.Sprintf("%d_%d", uid, i))
			if err != nil {
				log.Println(err)
			}
		}
	} else {
		for i := 0; i < 200; i++ {
			got, _ := cli.Get("w1")
			line := strings.Join(got, ",")
			log.Println(line)
			//line += "\n"
			//work <- line
			time.Sleep(10 * time.Millisecond)
		}
	}
}
