package client

import (
	"DRW/src/config"
	"DRW/src/rpc/cemm"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log"
	"strconv"
	"sync"
	"testing"
)

func TestConOp(t *testing.T) {
	cf := config.GetDefaultConfig()
	var wg sync.WaitGroup
	for i := 1; i <= cf.ClientCnt; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			conOp(idx, cf)
		}(i)
	}
	wg.Wait()
}

func conOp(c int, cf *config.Config) {
	conn, err := grpc.NewClient("127.0.0.1:19090", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	cli := NewEMMClient(c, cf, cemm.NewCEMMClient(conn))

	if c == 1 {
		for i := 1; i <= 300; i++ {
			err := cli.Add("key1", "value1_"+strconv.Itoa(i))
			if err != nil {
				log.Println(err)
				return
			}
			//time.Sleep(10 * time.Millisecond)
		}

	} else if c == 2 {
		for i := 1; i <= 300; i++ {
			err := cli.Add("key1", "value1_"+strconv.Itoa(i+300))
			if err != nil {
				log.Println(err)
				return
			}
			//time.Sleep(10 * time.Millisecond)
		}

	} else {
		for i := 1; i <= 10; i++ {
			//delay := 10 + rand.Intn(20)
			//time.Sleep(time.Duration(delay) * time.Millisecond)
			res, _ := cli.Get("key1")
			log.Printf("cli%d search key1 got :%v\n", c, res)
		}
	}

}

func TestName(t *testing.T) {
	conn, err := grpc.NewClient("127.0.0.1:19091", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	cf := config.GetDefaultConfig()
	cli := NewEMMClient(1, cf, cemm.NewCEMMClient(conn))
	for i := 1; i <= 2000; i++ {
		_, _ = cli.Get("key1")
		//log.Printf("cli%d search key1 got :%v\n", 1, res)
	}

}

func TestN(t *testing.T) {
	fmt.Println(genSt(genKwHash("key1"), 40000)[:3])
}
