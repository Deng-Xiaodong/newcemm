package server

import (
	"DRW/src/rpc/cemm"
	"context"
	"encoding/binary"
	"errors"
	"github.com/dgraph-io/badger/v4"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"io"
	"log"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

type AtomicCounter struct {
	value uint32
}

func NewAtomicCounter(v uint32) *AtomicCounter {
	return &AtomicCounter{v}
}
func (ac *AtomicCounter) Load() uint32 {
	return atomic.LoadUint32(&ac.value)
}
func (ac *AtomicCounter) FetchAndInc() uint32 {
	for {
		old := atomic.LoadUint32(&ac.value)
		if atomic.CompareAndSwapUint32(&ac.value, old, old+1) {
			return old
		}
	}
}

type Cache interface {
	Read(key []byte) ([]byte, error)
	Write(key []byte, value []byte) error
}
type Memory struct {
	Db sync.Map
}

func (m *Memory) Read(key []byte) ([]byte, error) {
	value, ok := m.Db.Load(string(key))
	if !ok {
		return nil, nil
	}
	return value.([]byte), nil
}

func (m *Memory) Write(key []byte, value []byte) error {
	m.Db.Store(string(key), value)
	return nil
}

type Disk struct {
	Db *badger.DB
}

func (d *Disk) Read(key []byte) ([]byte, error) {
	// 3. 读取数据
	var value []byte
	return value, d.Db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(key)
		if err != nil {
			if errors.Is(err, badger.ErrKeyNotFound) {
				return nil
			}
			return err
		}
		if item != nil {
			value, err = item.ValueCopy(nil)
		}
		return err
	})
}

func (d *Disk) Write(key []byte, value []byte) error {
	// 2. 写入数据
	return d.Db.Update(func(txn *badger.Txn) error {
		var err error
		err = txn.Set(key, value)
		if err != nil {
			return err
		}
		return nil
	})
}

type EMMServer struct {
	cemm.UnimplementedCEMMServer

	cnt      *AtomicCounter
	round    map[string]*AtomicCounter
	db       Cache
	workChan chan int64
	tc       int
}

func (s *EMMServer) Init(stream grpc.ClientStreamingServer[cemm.InitRequest, emptypb.Empty]) error {
	for {
		req, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		stag := string(req.Hw)
		if stag != "" {
			s.round[stag] = NewAtomicCounter(1)
		}

		for _, data := range req.Nodes {
			err = s.db.Write(data.Addr, data.Node)
			if err != nil {
				return err
			}
		}

	}
}

func NewEMMServer(db Cache, wc chan int64) *EMMServer {
	s := &EMMServer{
		db:       db,
		cnt:      NewAtomicCounter(2),
		round:    make(map[string]*AtomicCounter),
		workChan: wc,
	}
	return s
}

func (s *EMMServer) Get(in *cemm.GetRequest, stream grpc.ServerStreamingServer[cemm.GetReply]) error {
	//进入线性化点
	countGet := s.cnt.FetchAndInc()

	//tag := in.Tag
	st := in.St
	var data []byte

	var count uint32
	now := time.Now()
	for {
		//addr := utils.H1(string(slices.Concat(tag, st)))

		//value, err := s.db.Read(addr)
		value, err := s.db.Read(st)
		if err != nil {
			log.Printf("EDB read  error: %v", err)
			return err
		}
		if value == nil {
			break
		}

		//检查可见性
		count = binary.BigEndian.Uint32(value[:4])
		//_ = count
		data = value[36:]
		st = value[4:36]
		//st = utils.Xor(value[4:36], utils.H2(string(slices.Concat(tag, st))))
		if count == 0 || count > countGet {
			//log.Printf("get miss node with count   %v", count)
			continue
		}
		err = stream.Send(&cemm.GetReply{Node: slices.Clone(data)})
		if err != nil {
			log.Printf("server send error: %v", err)
			return err
		}
	}
	if s.tc%100 == 0 {
		s.workChan <- time.Since(now).Microseconds()
	}
	s.tc++
	return nil
}

func (s *EMMServer) GetOrIncRound(ctx context.Context, in *cemm.RoundRequest) (*cemm.RoundReply, error) {
	stag := string(in.Hw)
	t, ok := s.round[stag]
	if !ok || t == nil {
		log.Println("EDB get round error: init error")
		return nil, errors.New("init error")
	}

	rly := &cemm.RoundReply{}
	if in.Op {
		//查询
		rly.Round = t.FetchAndInc()
	} else {
		//添加
		rly.Round = t.Load()
	}
	return rly, nil
}

func (s *EMMServer) Add(ctx context.Context, in *cemm.AddRequest) (*emptypb.Empty, error) {

	err := s.db.Write(in.Next.Addr, in.Next.Node)
	if err != nil {
		log.Println("EDB write next error:", err)
		return &emptypb.Empty{}, err
	}

	err = s.db.Write(in.PreNext.Addr, in.PreNext.Node)
	if err != nil {
		log.Println("EDB write preNext error:", err)
		return &emptypb.Empty{}, err
	}

	fullNode(s.cnt.FetchAndInc(), in.Next.Node)
	//线性化点
	log.Println("EDB add node with count :", binary.BigEndian.Uint32(in.Next.Node[:4]))
	for s.db.Write(in.Next.Addr, in.Next.Node) != nil {
	}

	return &emptypb.Empty{}, nil
}

// 功能函数
func parseNode(node []byte) (st, data []byte) {
	st = node[:32]
	data = node[32:]
	return
}
func fullNode(count uint32, node []byte) {
	binary.BigEndian.PutUint32(node[:4], count)
}
