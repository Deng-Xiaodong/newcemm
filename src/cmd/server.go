package main

import (
	"DRW/src/rpc/cemm"
	"DRW/src/server"
	"fmt"
	"github.com/dgraph-io/badger/v4"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func main() {

	dbType := "memory"
	if len(os.Args) == 2 {
		dbType = os.Args[1]
	}
	var db server.Cache
	switch dbType {
	case "memory":
		db = &server.Memory{Db: sync.Map{}}
	case "disk":
		// 1. 打开数据库
		opts := badger.DefaultOptions("databases") // 数据存储在当前目录的 databases 文件夹中
		bg, errDb := badger.Open(opts)
		if errDb != nil {
			log.Fatal("Failed to open database: ", errDb)
		}
		db = &server.Disk{Db: bg}
	default:
	}

	var keepAliveArgs = keepalive.ServerParameters{
		Time:             10 * time.Second,
		Timeout:          20 * time.Second,
		MaxConnectionAge: 30 * time.Second,
	}
	listener, errLis := net.Listen("tcp", ":19091")
	if errLis != nil {
		log.Fatalf("failed to listen: %v", errLis)
	}
	s := grpc.NewServer(
		grpc.KeepaliveParams(keepAliveArgs),
		grpc.MaxSendMsgSize(1024*1024*1024),
		grpc.MaxRecvMsgSize(1024*1024*1024),
	)
	f, err := os.OpenFile("data/cost.txt", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	workChan := make(chan int64, 1000)
	go func() {
		for cost := range workChan {
			_, err = f.WriteString(fmt.Sprintf("%d\n", cost))
			if err != nil {
				log.Println(err)
			}
		}
	}()
	es := server.NewEMMServer(db, workChan)
	cemm.RegisterCEMMServer(s, es)

	reflection.Register(s)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		fmt.Printf("GraceFullyExit has exited, sig:%v\n", sig)
		s.GracefulStop()
	}()
	if err := s.Serve(listener); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}

}
